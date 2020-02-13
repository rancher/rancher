import ast
import os
import logging
import sys

from .common import run_command, run_command_with_stderr

logging.basicConfig(stream=sys.stdout,
                    level=os.environ.get("LOGLEVEL", "INFO"),
                    format='%(asctime)s - %(name)s - '
                           '[%(levelname)5s]: %(message)s')
DEFAULT_CONTEXT = os.environ.get('DEFAULT_CONTEXT', None)


class RancherCli:
    DEBUG_LOGS = ast.literal_eval(
        os.environ.get('DEBUG', "False").capitalize())
    log = logging.getLogger(__name__)

    @classmethod
    def run_command(cls, command, expect_error=False):
        command = "rancherctl {}".format(command)
        if cls.DEBUG_LOGS:
            cls.log.debug("run cmd:\t%s", command)
        if expect_error:
            result = run_command_with_stderr(command, log_out=False)
        else:
            result = run_command(command, log_out=False)
        if cls.DEBUG_LOGS:
            cls.log.debug("returns:\t%s", result)
        return result

    def set_log_level(self, level):
        self.log.setLevel(level)

    def login(self, url, token, **kwargs):
        context = kwargs.get("context", DEFAULT_CONTEXT)
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
