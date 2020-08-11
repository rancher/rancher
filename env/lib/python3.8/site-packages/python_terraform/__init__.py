# -*- coding: utf-8 -*-
# above is for compatibility of python2.7.11

import subprocess
import os
import sys
import json
import logging
import tempfile

from python_terraform.tfstate import Tfstate


try:  # Python 2.7+
    from logging import NullHandler
except ImportError:
    class NullHandler(logging.Handler):
        def emit(self, record):
            pass

log = logging.getLogger(__name__)
log.addHandler(NullHandler())


class IsFlagged:
    pass


class IsNotFlagged:
    pass


class TerraformCommandError(subprocess.CalledProcessError):
  def __init__(self, ret_code, cmd, out, err):
      super(TerraformCommandError, self).__init__(ret_code, cmd)
      self.out = out
      self.err = err

class Terraform(object):
    """
    Wrapper of terraform command line tool
    https://www.terraform.io/
    """

    def __init__(self, working_dir=None,
                 targets=None,
                 state=None,
                 variables=None,
                 parallelism=None,
                 var_file=None,
                 terraform_bin_path=None,
                 is_env_vars_included=True, 
                 ):
        """
        :param working_dir: the folder of the working folder, if not given,
                            will be current working folder
        :param targets: list of target
                        as default value of apply/destroy/plan command
        :param state: path of state file relative to working folder,
                    as a default value of apply/destroy/plan command
        :param variables: default variables for apply/destroy/plan command,
                        will be override by variable passing by apply/destroy/plan method
        :param parallelism: default parallelism value for apply/destroy command
        :param var_file: passed as value of -var-file option,
                could be string or list, list stands for multiple -var-file option
        :param terraform_bin_path: binary path of terraform
        :type is_env_vars_included: bool
        :param is_env_vars_included: included env variables when calling terraform cmd
        """
        self.is_env_vars_included = is_env_vars_included
        self.working_dir = working_dir
        self.state = state
        self.targets = [] if targets is None else targets
        self.variables = dict() if variables is None else variables
        self.parallelism = parallelism
        self.terraform_bin_path = terraform_bin_path \
            if terraform_bin_path else 'terraform'
        self.var_file = var_file
        self.temp_var_files = VariableFiles()

        # store the tfstate data
        self.tfstate = None
        self.read_state_file(self.state)

    def __getattr__(self, item):
        def wrapper(*args, **kwargs):
            cmd_name = str(item)
            if cmd_name.endswith('_cmd'):
                cmd_name = cmd_name[:-4]
            logging.debug('called with %r and %r' % (args, kwargs))
            return self.cmd(cmd_name, *args, **kwargs)

        return wrapper

    def apply(self, dir_or_plan=None, input=False, skip_plan=False, no_color=IsFlagged,
              **kwargs):
        """
        refer to https://terraform.io/docs/commands/apply.html
        no-color is flagged by default
        :param no_color: disable color of stdout
        :param input: disable prompt for a missing variable
        :param dir_or_plan: folder relative to working folder
        :param skip_plan: force apply without plan (default: false)
        :param kwargs: same as kwags in method 'cmd'
        :returns return_code, stdout, stderr
        """
        default = kwargs
        default['input'] = input
        default['no_color'] = no_color
        default['auto-approve'] = (skip_plan == True)
        option_dict = self._generate_default_options(default)
        args = self._generate_default_args(dir_or_plan)
        return self.cmd('apply', *args, **option_dict)

    def _generate_default_args(self, dir_or_plan):
        return [dir_or_plan] if dir_or_plan else []

    def _generate_default_options(self, input_options):
        option_dict = dict()
        option_dict['state'] = self.state
        option_dict['target'] = self.targets
        option_dict['var'] = self.variables
        option_dict['var_file'] = self.var_file
        option_dict['parallelism'] = self.parallelism
        option_dict['no_color'] = IsFlagged
        option_dict['input'] = False
        option_dict.update(input_options)
        return option_dict

    def destroy(self, dir_or_plan=None, force=IsFlagged, **kwargs):
        """
        refer to https://www.terraform.io/docs/commands/destroy.html
        force/no-color option is flagged by default
        :return: ret_code, stdout, stderr
        """
        default = kwargs
        default['force'] = force
        options = self._generate_default_options(default)
        args = self._generate_default_args(dir_or_plan)
        return self.cmd('destroy', *args, **options)

    def plan(self, dir_or_plan=None, detailed_exitcode=IsFlagged, **kwargs):
        """
        refer to https://www.terraform.io/docs/commands/plan.html
        :param detailed_exitcode: Return a detailed exit code when the command exits.
        :param dir_or_plan: relative path to plan/folder
        :param kwargs: options
        :return: ret_code, stdout, stderr
        """
        options = kwargs
        options['detailed_exitcode'] = detailed_exitcode
        options = self._generate_default_options(options)
        args = self._generate_default_args(dir_or_plan)
        return self.cmd('plan', *args, **options)

    def init(self, dir_or_plan=None, backend_config=None,
             reconfigure=IsFlagged, backend=True, **kwargs):
        """
        refer to https://www.terraform.io/docs/commands/init.html

        By default, this assumes you want to use backend config, and tries to
        init fresh. The flags -reconfigure and -backend=true are default.

        :param dir_or_plan: relative path to the folder want to init
        :param backend_config: a dictionary of backend config options. eg.
                t = Terraform()
                t.init(backend_config={'access_key': 'myaccesskey', 
                'secret_key': 'mysecretkey', 'bucket': 'mybucketname'})
        :param reconfigure: whether or not to force reconfiguration of backend
        :param backend: whether or not to use backend settings for init
        :param kwargs: options
        :return: ret_code, stdout, stderr
        """
        options = kwargs
        options['backend_config'] = backend_config
        options['reconfigure'] = reconfigure
        options['backend'] = backend
        options = self._generate_default_options(options)
        args = self._generate_default_args(dir_or_plan)
        return self.cmd('init', *args, **options)

    def generate_cmd_string(self, cmd, *args, **kwargs):
        """
        for any generate_cmd_string doesn't written as public method of terraform

        examples:
        1. call import command,
        ref to https://www.terraform.io/docs/commands/import.html
        --> generate_cmd_string call:
                terraform import -input=true aws_instance.foo i-abcd1234
        --> python call:
                tf.generate_cmd_string('import', 'aws_instance.foo', 'i-abcd1234', input=True)

        2. call apply command,
        --> generate_cmd_string call:
                terraform apply -var='a=b' -var='c=d' -no-color the_folder
        --> python call:
                tf.generate_cmd_string('apply', the_folder, no_color=IsFlagged, var={'a':'b', 'c':'d'})

        :param cmd: command and sub-command of terraform, seperated with space
                    refer to https://www.terraform.io/docs/commands/index.html
        :param args: arguments of a command
        :param kwargs: same as kwags in method 'cmd'
        :return: string of valid terraform command
        """
        cmds = cmd.split()
        cmds = [self.terraform_bin_path] + cmds

        for option, value in kwargs.items():
            if '_' in option:
                option = option.replace('_', '-')

            if type(value) is list:
                for sub_v in value:
                    cmds += ['-{k}={v}'.format(k=option, v=sub_v)]
                continue

            if type(value) is dict:
                if 'backend-config' in option:
                    for bk, bv in value.items():
                        cmds += ['-backend-config={k}={v}'.format(k=bk, v=bv)]
                    continue

                # since map type sent in string won't work, create temp var file for
                # variables, and clean it up later
                else:
                    filename = self.temp_var_files.create(value)
                    cmds += ['-var-file={0}'.format(filename)]
                    continue

            # simple flag,
            if value is IsFlagged:
                cmds += ['-{k}'.format(k=option)]
                continue

            if value is None or value is IsNotFlagged:
                continue

            if type(value) is bool:
                value = 'true' if value else 'false'

            cmds += ['-{k}={v}'.format(k=option, v=value)]

        cmds += args
        return cmds

    def cmd(self, cmd, *args, **kwargs):
        """
        run a terraform command, if success, will try to read state file
        :param cmd: command and sub-command of terraform, seperated with space
                    refer to https://www.terraform.io/docs/commands/index.html
        :param args: arguments of a command
        :param kwargs:  any option flag with key value without prefixed dash character
                if there's a dash in the option name, use under line instead of dash,
                    ex. -no-color --> no_color
                if it's a simple flag with no value, value should be IsFlagged
                    ex. cmd('taint', allow＿missing=IsFlagged)
                if it's a boolean value flag, assign True or false
                if it's a flag could be used multiple times, assign list to it's value
                if it's a "var" variable flag, assign dictionary to it
                if a value is None, will skip this option
                if the option 'capture_output' is passed (with any value other than
                    True), terraform output will be printed to stdout/stderr and
                    "None" will be returned as out and err.
                if the option 'raise_on_error' is passed (with any value that evaluates to True),
                    and the terraform command returns a nonzerop return code, then
                    a TerraformCommandError exception will be raised. The exception object will
                    have the following properties:
                      returncode: The command's return code
                      out: The captured stdout, or None if not captured
                      err: The captured stderr, or None if not captured
        :return: ret_code, out, err
        """
        capture_output = kwargs.pop('capture_output', True)
        raise_on_error = kwargs.pop('raise_on_error', False)
        if capture_output is True:
            stderr = subprocess.PIPE
            stdout = subprocess.PIPE
        else:
            stderr = sys.stderr
            stdout = sys.stdout

        cmds = self.generate_cmd_string(cmd, *args, **kwargs)
        log.debug('command: {c}'.format(c=' '.join(cmds)))

        working_folder = self.working_dir if self.working_dir else None

        environ_vars = {}
        if self.is_env_vars_included:
            environ_vars = os.environ.copy()

        p = subprocess.Popen(cmds, stdout=stdout, stderr=stderr,
                             cwd=working_folder, env=environ_vars)

        synchronous = kwargs.pop('synchronous', True)
        if not synchronous:
            return p, None, None

        out, err = p.communicate()
        ret_code = p.returncode
        log.debug('output: {o}'.format(o=out))

        if ret_code == 0:
            self.read_state_file()
        else:
            log.warn('error: {e}'.format(e=err))

        self.temp_var_files.clean_up()
        if capture_output is True:
            out = out.decode('utf-8')
            err = err.decode('utf-8')
        else:
            out = None
            err = None

        if ret_code != 0 and raise_on_error:
            raise TerraformCommandError(
                ret_code, ' '.join(cmds), out=out, err=err)

        return ret_code, out, err


    def output(self, *args, **kwargs):
        """
        https://www.terraform.io/docs/commands/output.html

        Note that this method does not conform to the (ret_code, out, err) return convention. To use
        the "output" command with the standard convention, call "output_cmd" instead of
        "output".

        :param args:   Positional arguments. There is one optional positional
                       argument NAME; if supplied, the returned output text
                       will be the json for a single named output value.
        :param kwargs: Named options, passed to the command. In addition, 
                          'full_value': If True, and NAME is provided, then
                                        the return value will be a dict with
                                        "value', 'type', and 'sensitive'
                                        properties.
        :return: None, if an error occured
                 Output value as a string, if NAME is provided and full_value
                    is False or not provided
                 Output value as a dict with 'value', 'sensitive', and 'type' if
                    NAME is provided and full_value is True.
                 dict of named dicts each with 'value', 'sensitive', and 'type',
                    if NAME is not provided
        """
        full_value = kwargs.pop('full_value', False)
        name_provided = (len(args) > 0)
        kwargs['json'] = IsFlagged
        if not kwargs.get('capture_output', True) is True:
          raise ValueError('capture_output is required for this method')

        ret, out, err = self.output_cmd(*args, **kwargs)

        if ret != 0:
            return None

        out = out.lstrip()

        value = json.loads(out)

        if name_provided and not full_value:
            value = value['value']

        return value

    def read_state_file(self, file_path=None):
        """
        read .tfstate file
        :param file_path: relative path to working dir
        :return: states file in dict type
        """

        working_dir = self.working_dir or ''

        file_path = file_path or self.state or ''

        if not file_path:
            backend_path = os.path.join(file_path, '.terraform',
                                        'terraform.tfstate')

            if os.path.exists(os.path.join(working_dir, backend_path)):
                file_path = backend_path
            else:
                file_path = os.path.join(file_path, 'terraform.tfstate')

        file_path = os.path.join(working_dir, file_path)

        self.tfstate = Tfstate.load_file(file_path)

    def set_workspace(self, workspace):
        """
        set workspace
        :param workspace: the desired workspace.
        :return: status
        """

        return self.cmd('workspace' ,'select', workspace)  

    def create_workspace(self, workspace):
        """
        create workspace
        :param workspace: the desired workspace.
        :return: status
        """

        return self.cmd('workspace', 'new', workspace)     

    def delete_workspace(self, workspace):
        """
        delete workspace
        :param workspace: the desired workspace.
        :return: status
        """

        return self.cmd('workspace', 'delete', workspace)    

    def show_workspace(self):
        """
        show workspace
        :return: workspace
        """

        return self.cmd('workspace', 'show')  

    def __exit__(self, exc_type, exc_value, traceback):
        self.temp_var_files.clean_up()


class VariableFiles(object):
    def __init__(self):
        self.files = []

    def create(self, variables):
        with tempfile.NamedTemporaryFile('w+t', suffix='.tfvars.json', delete=False) as temp:
            log.debug('{0} is created'.format(temp.name))
            self.files.append(temp)
            log.debug(
                'variables wrote to tempfile: {0}'.format(str(variables)))
            temp.write(json.dumps(variables))
            file_name = temp.name

        return file_name

    def clean_up(self):
        for f in self.files:
            os.unlink(f.name)

        self.files = []
