import os
import cattle
import pytest
import gdapi
from docker import Client
from docker.utils import kwargs_from_env


ADMIN_HEADERS = dict(gdapi.HEADERS)
ADMIN_HEADERS['X-API-Project-Id'] = 'USER'


class CattleConfig(object):
    def __init__(self, url_env):
        self.url_env = url_env

    def _get_client(self):
        client = cattle.from_env(url=self.cattle_url(),
                                 cache=False,
                                 headers=ADMIN_HEADERS)
        assert client.valid()
        return client

    def cattle_url(self):
        return os.environ.get(self.url_env)

    def _get_setting(self, setting):
        client = self._get_client()
        return client.by_id_setting('1as!{0}'.format(setting))['activeValue']

    def assert_setting(self, setting, value):
        configured_setting = self._get_setting(setting)
        assert configured_setting == value


class DockerContainerTester(object):
    def __init__(self, assert_hostname=True):
        kwargs = kwargs_from_env(assert_hostname=assert_hostname)
        self.client = Client(**kwargs)
        self.containers = self._load_containers()

    def _load_containers(self):
        data = {}
        for container in self.client.containers():
            name = container['Names'][0][1:]
            data[name] = container

        return data

    def _get_container(self, container_name):
        '''
        This checks the objects container cache, refreshes
        if it does not exist. Then tries to return again.
        '''
        container = self.containers.get(container_name, None)
        if container is not None:
            return container
        else:
            self.containers = self._load_containers()
            return self.containers.get(container_name, None)

    def _get_process_commands(self, container_name):
        # for simplicity at this moment... I know user will be important
        commands = []
        container = self._get_container(container_name)
        if container is not None:
            top = self.client.top(container['Id'])

            try:
                idx = top['Titles'].index("COMMAND")
            except ValueError:
                idx = top['Titles'].index("CMD")

            processes = top['Processes']
            commands = [process[idx] for process in processes]

        return commands

    def assert_command_running(self, container_name, process_name):
        processes = self._get_process_commands(container_name)
        assert process_name in processes

    def assert_command_not_running(self, container_name, process_name):
        processes = self._get_process_commands(container_name)
        assert process_name not in processes


@pytest.fixture()
def docker_containers():
    return DockerContainerTester(assert_hostname=False)


@pytest.fixture()
def mysql_command():
    mysql_cmd = "/usr/sbin/mysqld --basedir=/usr --datadir=/var/lib/mysql" \
                " --plugin-dir=/usr/lib/mysql/plugin --user=mysql" \
                " --log-error=/var/log/mysql/error.log" \
                " --pid-file=/var/run/mysqld/mysqld.pid" \
                " --socket=/var/run/mysqld/mysqld.sock --port=3306"

    return mysql_cmd


@pytest.fixture()
def h2_cattle_config():
    return CattleConfig('CATTLE_H2DB_TEST_URL')


@pytest.fixture()
def mysql_link_cattle_config():
    return CattleConfig('CATTLE_MYSQL_LINK_TEST_URL')


@pytest.fixture()
def mysql_local_cattle_config():
    return CattleConfig('CATTLE_MYSQL_LOCAL_TEST_URL')


@pytest.fixture()
def mysql_manual_cattle_config():
    return CattleConfig('CATTLE_MYSQL_MANUAL_TEST_URL')


def test_h2_database_overrides_mysql(h2_cattle_config):
    h2_cattle_config.assert_setting('db.cattle.database', 'h2')


def test_mysql_link_database_db(mysql_link_cattle_config):
    mysql_link_cattle_config.assert_setting('db.cattle.database', 'mysql')


def test_mysql_local_database_db(mysql_local_cattle_config):
    mysql_local_cattle_config.assert_setting('db.cattle.database', 'mysql')


def test_mysql_manual_database_db(mysql_manual_cattle_config):
    mysql_manual_cattle_config.assert_setting('db.cattle.database', 'mysql')


def test_local_cattle_db_has_mysql_process(docker_containers, mysql_command):
    docker_containers.assert_command_running(
        'server_localmysqlcattle_1', mysql_command)


def test_link_cattle_no_mysql_process(docker_containers, mysql_command):
    docker_containers.assert_command_not_running(
        'server_mysqllinkcattle_1', mysql_command)


def test_manual_cattle_no_mysql_process(docker_containers, mysql_command):
    docker_containers.assert_command_not_running(
        'server_mysqlmanualcattle_1', mysql_command)


def test_h2_cattle_no_mysql_process(docker_containers, mysql_command):
    docker_containers.assert_command_not_running(
        'server_h2dbcattle_1', mysql_command)
