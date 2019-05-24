from flask import Flask, request
import os
import random
import requests
import socket
from string import ascii_letters, digits
from subprocess import call


TEMP_DIR = os.path.dirname(os.path.realpath(__file__)) + '/temp'
app = Flask(__name__)


def generate_random_file_name():
    name = ''.join(random.choice(ascii_letters + digits) for _ in list(range(35)))
    return "{0}/{1}.txt".format(TEMP_DIR, name)


@app.route('/')
def home():
    return "welcome to container-utils"

@app.route('/metadata/<path:path>', methods=['GET'])
def get_metadata(path):
    accept_type = request.headers.get('Accept')
    headers = {'Accept': accept_type} if accept_type else None
    url = "http://rancher-metadata/%s" % path
    try:
        response = requests.get(url=url, headers=headers)
    except Exception as e:
        return "Error: {0}".format(e), 400
    if not response.ok:
        return response.content, response.status_code
    return response.content, 200


@app.route('/hostname', methods=['GET'])
def get_hostname():
    return str(socket.gethostname())


@app.route('/env', methods=['GET'])
def get_environment_varable():
    if 'var' not in request.args:
        return "Required param 'var' is missing", 400

    var = request.args['var']
    if var not in os.environ:
        return "Not found '{0}' in environment variables".format(var), 404
    return str(os.environ[var])


@app.route('/proxy', methods=['GET'])
def proxy():
    url = request.args.get('url')
    link = request.args.get('link')
    port = request.args.get('port')
    path = request.args.get('path')

    if link is not None and port is not None and path is not None:
        link = link.upper()
        dest_port = os.environ.get(link + "_PORT_" + port + "_TCP_PORT")
        dest_host = os.environ.get(link + "_PORT_" + port + "_TCP_ADDR")
        err_msg = "Not found '{0}' in environment variables"
        if dest_port is None:
            return err_msg.format(dest_port), 404
        if dest_host is None:
            return err_msg.format(dest_host), 404
        url = 'http://{0}:{1}/{2}'.format(dest_host, dest_port, path)

    if url is None:
        return ("Required param missing: Either 'url', or all params "
                "'link', 'port' and 'path' are required"), 400
    try:
        response = requests.get(url=url)
    except Exception as e:
        return "Error: {0}".format(e), 400
    if not response.ok:
        return response.content, response.status_code
    return response.content, 200


@app.route('/dig', methods=['GET'])
def get_dig_info():
    if 'host' not in request.args:
        return "Required param 'host' is missing", 400
    host = request.args['host']

    temp_file = generate_random_file_name()
    try:
        with open(temp_file, 'w') as f:
            call(['dig', host, '+short'], stdout=f)

        with open(temp_file, 'r') as f:
            content = f.read()
    except Exception as e:
        content = "Error: {0}".format(e)
    finally:
        if os.path.isfile(temp_file):
            os.remove(temp_file)
    return content


@app.route('/ping', methods=['GET'])
def health_check():
    return 'ping'


if __name__ == '__main__':
    if not os.path.isdir(TEMP_DIR):
        os.makedirs(TEMP_DIR)
    app.run(debug=True, host='0.0.0.0')
