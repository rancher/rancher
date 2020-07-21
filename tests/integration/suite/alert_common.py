import requests

from flask import request
from threading import Thread


class MockServer(Thread):
    def __init__(self, port=5000):
        super().__init__()
        from flask import Flask
        self.port = port
        self.app = Flask(__name__)
        self.url = "http://127.0.0.1:%s" % self.port

        self.app.add_url_rule("/shutdown", view_func=self._shutdown_server)

    def _shutdown_server(self):
        from flask import request
        if 'werkzeug.server.shutdown' not in request.environ:
            raise RuntimeError('Not running the development server')
        request.environ['werkzeug.server.shutdown']()
        return 'Server shutting down...'

    def shutdown_server(self):
        requests.get("http://127.0.0.1:%s/shutdown" % self.port,
                     headers={'Connection': 'close'})
        self.join()

    def run(self):
        self.app.run(host='0.0.0.0', port=self.port, threaded=True)


class MockReceiveAlert(MockServer):

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

    def __init__(self, port):
        super().__init__(port)
        self.add_endpoints()


DINGTALK_MESSAGE = "Dingtalk setting validated"

MICROSOFTTEAMS_MESSAGE = "MicrosoftTeams setting validated"
