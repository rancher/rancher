import os
import logging
import sys
import time

from .common import run_command, run_command_with_stderr

logging.basicConfig(stream=sys.stdout,
                    level=os.environ.get("LOGLEVEL", "INFO"),
                    format='%(asctime)s - %(filename)s:%(funcName)s'
                           ':%(lineno)d - [%(levelname)5s]: %(message)s',
                    datefmt="%H:%M:%S")
DEFAULT_TIMEOUT = 60


class BaseCli:
    log = logging.getLogger(__name__)
    DEFAULT_CONTEXT = os.environ.get('DEFAULT_CONTEXT', None)

    @classmethod
    def run_command(cls, command, expect_error=False):
        command = "rancherctl {}".format(command)
        cls.log.debug("run cmd:\t%s", command)
        if expect_error:
            result = run_command_with_stderr(command, log_out=False)
        else:
            result = run_command(command, log_out=False)
        cls.log.debug("returns:\t%s", result)
        return result

    def set_log_level(self, level):
        self.log.setLevel(level)

    def login(self, url, token, **kwargs):
        context = kwargs.get("context", self.DEFAULT_CONTEXT)
        if context is None:
            raise ValueError("No context supplied for rancher login!")
        cmd = "login {} --token {} --context {} --skip-verify".format(
            url, token, context)
        self.run_command(cmd, expect_error=True)

    def switch_context(self, project_id):
        self.run_command("context switch {}".format(project_id),
                         expect_error=True)

    def get_context(self):
        result = self.run_command("context current")
        cluster_name = result[8:result.index(" ")].strip()
        project_name = result[result.index("Project:") + 8:].strip()
        return cluster_name, project_name

    def get_cluster_by_name(self, name):
        for c in self.get_clusters():
            if c["name"] == name:
                return c

    def get_current_cluster(self):
        for c in self.get_clusters():
            if c["current"]:
                return c

    def get_clusters(self):
        result = self.run_command("clusters ls --format '{{.Cluster.ID}}"
                                  "|{{.Cluster.Name}}|{{.Current}}|{{.Cluster.UUID}}'")
        clusters = []
        for c in result.splitlines():
            c = c.split("|")
            cluster = {
                "id": c[0],
                "name": c[1],
                "current": c[2] == "*",
                "uuid": c[3]
            }
            clusters.append(cluster)
        return clusters

    def wait_for_ready(self, command, val_to_check, **kwargs):
        timeout = kwargs.get("timeout", DEFAULT_TIMEOUT)
        condition_func = kwargs.get("condition_func",
                                    lambda val, l: val in l.splitlines())
        done = False
        start_time = time.time()
        while not done and time.time() - start_time < timeout:
            result = self.run_command(command)
            if condition_func(val_to_check, result):
                done = True
        return done
