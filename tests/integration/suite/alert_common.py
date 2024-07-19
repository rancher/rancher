from flask import request
from flask import Flask
from threading import Thread
from werkzeug.serving import make_server
import socket


def get_ip():
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        # doesn't even have to be reachable
        s.connect(('10.255.255.255', 1))
        IP = s.getsockname()[0]
    except Exception:
        IP = '127.0.0.1'
    finally:
        s.close()
    return IP


LOCAL_IP = get_ip()


class MockServer(Thread):
    def __init__(self, port=5000):
        self.port = port
        self.app = Flask(__name__)
        self.server = make_server(LOCAL_IP, self.port, self.app)
        self.url = "http://%s:%s" % (LOCAL_IP, self.port)
        self.thread = None

    def stop(self):
        self.server.shutdown()
        self.thread.join()

    def start(self):
        self.thread = Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()


class MockReceiveAlert(MockServer):
    def __init__(self, port):
        super().__init__(port)
        self.add_endpoints()

    def api_microsoft_teams(self):
        message = request.json.get("text")
        assert message == MICROSOFTTEAMS_MESSAGE
        return "success"

    def api_dingtalk(self, url):
        message = request.json.get("text")
        assert message.get('content') == DINGTALK_MESSAGE
        return '{"errcode":0,"errmsg":""}'

    def add_endpoints(self):
        self.app.add_url_rule("/microsoftTeams",
                              view_func=self.api_microsoft_teams,
                              methods=('POST',))
        self.app.add_url_rule("/dingtalk/<path:url>/",
                              view_func=self.api_dingtalk,
                              methods=('POST',))
        pass


DINGTALK_MESSAGE = "Dingtalk setting validated"

MICROSOFTTEAMS_MESSAGE = "MicrosoftTeams setting validated"
