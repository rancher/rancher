import time

from .common import get_user_client, random_test_name
from .cli_common import RancherCli, DEFAULT_CONTEXT

DEFAULT_TIMEOUT = 60

class CliProject(RancherCli):
    def __init__(self):
        self.initial_projects = self.get_current_projects()

    def create_project(self, name=None,
                       cluster_id=None, use_context=True):
        if name is None:
            name = random_test_name("ptest")
        if cluster_id is None:
            cluster = self.get_context()[0]
            cluster_id = self.get_cluster_by_name(cluster)["id"]
        self.run_command("projects create --cluster {} {}".format(cluster_id,
                                                                  name))
        project = None
        for p in self.get_current_projects():
            if p["name"] == name:
                project = p
                self.log.info("Project '%s' created successfully "
                              "in cluster '%s'", name, cluster_id)
                break
        if project is None:
            self.log.error("Failed to create project '%s' "
                           "in cluster '%s'", name, cluster_id)
            return project

        if use_context:
            self.log.info("Switching context to newly created project: "
                          "%s", name)
            for p in self.get_current_projects():
                if p["name"] == name:
                    self.switch_context(p["id"])
                    break
        return project

    def delete_project(self, name):
        self.run_command("projects rm {}".format(name))

    @classmethod
    def get_current_projects(cls):
        """This uses the Rancher Python Client to retrieve the current projects
        as there is not a CLI way to do this without passing stdin at the time
        of creation (2/13/2020, Rancher v2.3.5).
        Returns array of dictionaries containing id, name, clusterid, & uuid"""
        client = get_user_client()
        projects = client.list_project()
        current_projects = []
        for project in projects:
            p = {
                "id": project["id"],
                "name": project["name"],
                "clusterId": project["clusterId"],
                "state": project["state"],
                "uuid": project["uuid"]
            }
            current_projects.append(p)
        cls.log.debug("Projects: %s", current_projects)
        return current_projects

    def create_namespace(self, name=None):
        if name is None:
            name = random_test_name("nstest")
        self.run_command("namespace create {}".format(name))
        return name

    def delete_namespace(self, name):
        self.run_command("namespace delete {}".format(name))

        deleted = False
        self.log.debug("Waiting for the namespace to be deleted")
        start_time = time.time()
        while not deleted and time.time() - start_time < DEFAULT_TIMEOUT:
            namespaces = self.run_command("namespace ls -q")
            if name not in namespaces.splitlines():
                deleted = True

        if not deleted:
            ns = self.run_command("namespace ls --format '{{.Namespace.Name}}"
                                  "|{{.Namespace.State}}' "
                                  "| grep {}".format(name))
            self.log.warn("Namespace did not delete within timeout. "
                          "Status: %s", ns.split("|")[1])
            if not ns:
                return True
            return False
        return True

    def get_namespaces(self):
        namespaces = self.run_command("namespace ls --format "
                                      "'{{.Namespace.Name}}"
                                      "|{{.Namespace.State}}'")
        return namespaces.splitlines()

    def move_namespace(self, name, project_id):
        self.run_command("namespace move {} {}".format(name, project_id))
