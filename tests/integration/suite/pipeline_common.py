# flake8: noqa
import requests

from flask import jsonify
from threading import Thread


class MockServer(Thread):
    def __init__(self, port=5000):
        super().__init__()
        from flask import Flask
        self.port = port
        self.app = Flask(__name__)
        self.url = "http://localhost:%s" % self.port

        self.app.add_url_rule("/shutdown", view_func=self._shutdown_server)

    def _shutdown_server(self):
        from flask import request
        if 'werkzeug.server.shutdown' not in request.environ:
            raise RuntimeError('Not running the development server')
        request.environ['werkzeug.server.shutdown']()
        return 'Server shutting down...'

    def shutdown_server(self):
        requests.get("http://localhost:%s/shutdown" % self.port,
                     headers={'Connection': 'close'})
        self.join()

    def run(self):
        self.app.run(host='0.0.0.0', port=self.port, threaded=True)


class MockGithub(MockServer):

    def api_user(self):
        return jsonify(GITHUB_USER_PAYLOAD)

    def api_repos(self):
        return jsonify(GITHUB_REPOS_PAYLOAD)

    def api_file_content(self):
        return jsonify(GITHUB_FILE_CONTENT_PAYLOAD)

    def api_commit(self):
        return jsonify(GITHUB_COMMIT_PAYLOAD)

    def api_branch(self):
        return jsonify(GITHUB_BRANCH_PAYLOAD)

    def api_access_token(self):
        return jsonify({'access_token': 'test_token', 'token_type': 'bearer'})

    def add_endpoints(self):
        self.app.add_url_rule("/login/oauth/access_token",
                              view_func=self.api_access_token,
                              methods=('POST',))
        self.app.add_url_rule("/api/v3/user", view_func=self.api_user)
        self.app.add_url_rule("/api/v3/user/repos", view_func=self.api_repos)
        self.app.add_url_rule(
            "/api/v3/repos/octocat/Hello-World/contents/.rancher-pipeline.yml",
            view_func=self.api_file_content)
        self.app.add_url_rule(
            "/api/v3/repos/octocat/Hello-World/commits/master",
            view_func=self.api_commit)
        self.app.add_url_rule("/api/v3/repos/octocat/Hello-World/branches",
                              view_func=self.api_branch)
        pass

    def __init__(self, port):
        super().__init__(port)
        self.add_endpoints()


GITHUB_USER_PAYLOAD = {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "https://github.com/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "https://github.com/api/v3/users/octocat",
    "html_url": "https://github.com/octocat",
    "followers_url": "https://github.com/api/v3/users/octocat/followers",
    "following_url": "https://github.com/api/v3/users/octocat/following{/other_user}",
    "gists_url": "https://github.com/api/v3/users/octocat/gists{/gist_id}",
    "starred_url": "https://github.com/api/v3/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "https://github.com/api/v3/users/octocat/subscriptions",
    "organizations_url": "https://github.com/api/v3/users/octocat/orgs",
    "repos_url": "https://github.com/api/v3/users/octocat/repos",
    "events_url": "https://github.com/api/v3/users/octocat/events{/privacy}",
    "received_events_url": "https://github.com/api/v3/users/octocat/received_events",
    "type": "User",
    "site_admin": False,
    "name": "monalisa octocat",
    "company": "GitHub",
    "blog": "https://github.com/blog",
    "location": "San Francisco",
    "email": "octocat@github.com",
    "hireable": False,
    "bio": "There once was...",
    "public_repos": 2,
    "public_gists": 1,
    "followers": 20,
    "following": 0,
    "created_at": "2008-01-14T04:33:35Z",
    "updated_at": "2008-01-14T04:33:35Z"
}

GITHUB_REPOS_PAYLOAD = [
    {
        "id": 1296269,
        "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
        "name": "Hello-World",
        "full_name": "octocat/Hello-World",
        "owner": {
            "login": "octocat",
            "id": 1,
            "node_id": "MDQ6VXNlcjE=",
            "avatar_url": "https://github.com/images/error/octocat_happy.gif",
            "gravatar_id": "",
            "url": "https://github.com/api/v3/users/octocat",
            "html_url": "https://github.com/octocat",
            "followers_url": "https://github.com/api/v3/users/octocat/followers",
            "following_url": "https://github.com/api/v3/users/octocat/following{/other_user}",
            "gists_url": "https://github.com/api/v3/users/octocat/gists{/gist_id}",
            "starred_url": "https://github.com/api/v3/users/octocat/starred{/owner}{/repo}",
            "subscriptions_url": "https://github.com/api/v3/users/octocat/subscriptions",
            "organizations_url": "https://github.com/api/v3/users/octocat/orgs",
            "repos_url": "https://github.com/api/v3/users/octocat/repos",
            "events_url": "https://github.com/api/v3/users/octocat/events{/privacy}",
            "received_events_url": "https://github.com/api/v3/users/octocat/received_events",
            "type": "User",
            "site_admin": False
        },
        "private": False,
        "html_url": "https://github.com/octocat/Hello-World",
        "description": "This your first repo!",
        "fork": False,
        "url": "https://github.com/api/v3/repos/octocat/Hello-World",
        "archive_url": "https://github.com/api/v3/repos/octocat/Hello-World/{archive_format}{/ref}",
        "assignees_url": "https://github.com/api/v3/repos/octocat/Hello-World/assignees{/user}",
        "blobs_url": "https://github.com/api/v3/repos/octocat/Hello-World/git/blobs{/sha}",
        "branches_url": "https://github.com/api/v3/repos/octocat/Hello-World/branches{/branch}",
        "collaborators_url": "https://github.com/api/v3/repos/octocat/Hello-World/collaborators{/collaborator}",
        "comments_url": "https://github.com/api/v3/repos/octocat/Hello-World/comments{/number}",
        "commits_url": "https://github.com/api/v3/repos/octocat/Hello-World/commits{/sha}",
        "compare_url": "https://github.com/api/v3/repos/octocat/Hello-World/compare/{base}...{head}",
        "contents_url": "https://github.com/api/v3/repos/octocat/Hello-World/contents/{+path}",
        "contributors_url": "https://github.com/api/v3/repos/octocat/Hello-World/contributors",
        "deployments_url": "https://github.com/api/v3/repos/octocat/Hello-World/deployments",
        "downloads_url": "https://github.com/api/v3/repos/octocat/Hello-World/downloads",
        "events_url": "https://github.com/api/v3/repos/octocat/Hello-World/events",
        "forks_url": "https://github.com/api/v3/repos/octocat/Hello-World/forks",
        "git_commits_url": "https://github.com/api/v3/repos/octocat/Hello-World/git/commits{/sha}",
        "git_refs_url": "https://github.com/api/v3/repos/octocat/Hello-World/git/refs{/sha}",
        "git_tags_url": "https://github.com/api/v3/repos/octocat/Hello-World/git/tags{/sha}",
        "git_url": "git:github.com/octocat/Hello-World.git",
        "issue_comment_url": "https://github.com/api/v3/repos/octocat/Hello-World/issues/comments{/number}",
        "issue_events_url": "https://github.com/api/v3/repos/octocat/Hello-World/issues/events{/number}",
        "issues_url": "https://github.com/api/v3/repos/octocat/Hello-World/issues{/number}",
        "keys_url": "https://github.com/api/v3/repos/octocat/Hello-World/keys{/key_id}",
        "labels_url": "https://github.com/api/v3/repos/octocat/Hello-World/labels{/name}",
        "languages_url": "https://github.com/api/v3/repos/octocat/Hello-World/languages",
        "merges_url": "https://github.com/api/v3/repos/octocat/Hello-World/merges",
        "milestones_url": "https://github.com/api/v3/repos/octocat/Hello-World/milestones{/number}",
        "notifications_url": "https://github.com/api/v3/repos/octocat/Hello-World/notifications{?since,all,participating}",
        "pulls_url": "https://github.com/api/v3/repos/octocat/Hello-World/pulls{/number}",
        "releases_url": "https://github.com/api/v3/repos/octocat/Hello-World/releases{/id}",
        "ssh_url": "git@github.com:octocat/Hello-World.git",
        "stargazers_url": "https://github.com/api/v3/repos/octocat/Hello-World/stargazers",
        "statuses_url": "https://github.com/api/v3/repos/octocat/Hello-World/statuses/{sha}",
        "subscribers_url": "https://github.com/api/v3/repos/octocat/Hello-World/subscribers",
        "subscription_url": "https://github.com/api/v3/repos/octocat/Hello-World/subscription",
        "tags_url": "https://github.com/api/v3/repos/octocat/Hello-World/tags",
        "teams_url": "https://github.com/api/v3/repos/octocat/Hello-World/teams",
        "trees_url": "https://github.com/api/v3/repos/octocat/Hello-World/git/trees{/sha}",
        "clone_url": "https://github.com/octocat/Hello-World.git",
        "mirror_url": "git:git.example.com/octocat/Hello-World",
        "hooks_url": "https://github.com/api/v3/repos/octocat/Hello-World/hooks",
        "svn_url": "https://svn.github.com/octocat/Hello-World",
        "homepage": "https://github.com",
        "language": None,
        "forks_count": 9,
        "stargazers_count": 80,
        "watchers_count": 80,
        "size": 108,
        "default_branch": "master",
        "open_issues_count": 0,
        "topics": [
            "octocat",
            "atom",
            "electron",
            "API"
        ],
        "has_issues": True,
        "has_projects": True,
        "has_wiki": True,
        "has_pages": False,
        "has_downloads": True,
        "archived": False,
        "pushed_at": "2011-01-26T19:06:43Z",
        "created_at": "2011-01-26T19:01:12Z",
        "updated_at": "2011-01-26T19:14:43Z",
        "permissions": {
            "admin": True,
            "push": True,
            "pull": True
        },
        "allow_rebase_merge": True,
        "allow_squash_merge": True,
        "allow_merge_commit": True,
        "subscribers_count": 42,
        "network_count": 0,
        "license": {
            "key": "mit",
            "name": "MIT License",
            "spdx_id": "MIT",
            "url": "https://github.com/api/v3/licenses/mit",
            "node_id": "MDc6TGljZW5zZW1pdA=="
        }
    }
]

GITHUB_FILE_CONTENT_PAYLOAD = {
    "name": ".rancher-pipeline.yml",
    "path": ".rancher-pipeline.yml",
    "sha": "e849c8954bad15cdccd309d3d434b7580e3246ce",
    "size": 881,
    "url": "https://github.com/api/v3/repos/octocat/Hello-World/contents/.rancher-pipeline.yml?ref=master",
    "html_url": "https://github.com/octocat/Hello-World/blob/master/.rancher-pipeline.yml",
    "git_url": "https://github.com/api/v3/repos/octocat/Hello-World/git/blobs/e849c8954bad15cdccd309d3d434b7580e3246ce",
    "type": "file",
    "content": "c3RhZ2VzOgotIG5hbWU6IENvZGVjZXB0aW9uIHRlc3QKICBzdGVwczoKICAt\nIHJ1blNjcmlwdENvbmZpZzoKICAgICAgaW1hZ2U6IHBocDo3LjIKICAgICAg\nc2hlbGxTY3JpcHQ6IHwtCiAgICAgICAgYXB0LWdldCB1cGRhdGUKICAgICAg\nICBhcHQtZ2V0IGluc3RhbGwgLXkgLS1uby1pbnN0YWxsLXJlY29tbWVuZHMg\nZ2l0IHppcCBsaWJzcWxpdGUzLWRldiB6bGliMWctZGV2CiAgICAgICAgZG9j\na2VyLXBocC1leHQtaW5zdGFsbCB6aXAKICAgICAgICBjdXJsIC0tc2lsZW50\nIC0tc2hvdy1lcnJvciBodHRwczovL2dldGNvbXBvc2VyLm9yZy9pbnN0YWxs\nZXIgfCBwaHAKICAgICAgICAuL2NvbXBvc2VyLnBoYXIgaW5zdGFsbCAtbiAt\nLXByZWZlci1kaXN0CiAgICAgICAgdG91Y2ggc3RvcmFnZS90ZXN0aW5nLnNx\nbGl0ZSBzdG9yYWdlL2RhdGFiYXNlLnNxbGl0ZQogICAgICAgIGNwIC5lbnYu\ndGVzdGluZyAuZW52CiAgICAgICAgcGhwIGFydGlzYW4gbWlncmF0ZQogICAg\nICAgIHBocCBhcnRpc2FuIG1pZ3JhdGUgLS1lbnY9dGVzdGluZyAtLWRhdGFi\nYXNlPXNxbGl0ZV90ZXN0aW5nIC0tZm9yY2UKICAgICAgICAuL3ZlbmRvci9i\naW4vY29kZWNlcHQgYnVpbGQKICAgICAgICAuL3ZlbmRvci9iaW4vY29kZWNl\ncHQgcnVuCi0gbmFtZTogUHVibGlzaCBpbWFnZQogIHN0ZXBzOgogIC0gcHVi\nbGlzaEltYWdlQ29uZmlnOgogICAgICBkb2NrZXJmaWxlUGF0aDogLi9Eb2Nr\nZXJmaWxlCiAgICAgIGJ1aWxkQ29udGV4dDogLgogICAgICB0YWc6IHBocC1l\neGFtcGxlOiR7Q0lDRF9FWEVDVVRJT05fU0VRVUVOQ0V9Ci0gbmFtZTogRGVw\nbG95CiAgc3RlcHM6CiAgLSBhcHBseVlhbWxDb25maWc6CiAgICAgIHBhdGg6\nIC4vZGVwbG95L2RlcGxveW1lbnQueWFtbAo=\n",
    "encoding": "base64",
    "_links": {
        "self": "https://github.com/api/v3/repos/octocat/Hello-World/contents/.rancher-pipeline.yml?ref=master",
        "git": "https://github.com/api/v3/repos/octocat/Hello-World/git/blobs/e849c8954bad15cdccd309d3d434b7580e3246ce",
        "html": "https://github.com/octocat/Hello-World/blob/master/.rancher-pipeline.yml"
    }
}

GITHUB_COMMIT_PAYLOAD = {
    "sha": "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
    "node_id": "MDY6Q29tbWl0MTI5NjI2OTo3ZmQxYTYwYjAxZjkxYjMxNGY1OTk1NWE0ZTRkNGU4MGQ4ZWRmMTFk",
    "commit": {
        "author": {
            "name": "The Octocat",
            "email": "octocat@nowhere.com",
            "date": "2012-03-06T23:06:50Z"
        },
        "committer": {
            "name": "The Octocat",
            "email": "octocat@nowhere.com",
            "date": "2012-03-06T23:06:50Z"
        },
        "message": "Merge pull request #6 from Spaceghost/patch-1\n\nNew line at end of file.",
        "tree": {
            "sha": "b4eecafa9be2f2006ce1b709d6857b07069b4608",
            "url": "https://github.com/api/v3/repos/octocat/Hello-World/git/trees/b4eecafa9be2f2006ce1b709d6857b07069b4608"
        },
        "url": "https://github.com/api/v3/repos/octocat/Hello-World/git/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
        "comment_count": 55,
        "verification": {
            "verified": False,
            "reason": "unsigned",
            "signature": None,
            "payload": None
        }
    },
    "url": "https://github.com/api/v3/repos/octocat/Hello-World/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
    "html_url": "https://github.com/octocat/Hello-World/commit/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
    "comments_url": "https://github.com/api/v3/repos/octocat/Hello-World/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d/comments",
    "author": {
        "login": "octocat",
        "id": 583231,
        "node_id": "MDQ6VXNlcjU4MzIzMQ==",
        "avatar_url": "https://avatars3.githubusercontent.com/u/583231?v=4",
        "gravatar_id": "",
        "url": "https://github.com/api/v3/users/octocat",
        "html_url": "https://github.com/octocat",
        "followers_url": "https://github.com/api/v3/users/octocat/followers",
        "following_url": "https://github.com/api/v3/users/octocat/following{/other_user}",
        "gists_url": "https://github.com/api/v3/users/octocat/gists{/gist_id}",
        "starred_url": "https://github.com/api/v3/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "https://github.com/api/v3/users/octocat/subscriptions",
        "organizations_url": "https://github.com/api/v3/users/octocat/orgs",
        "repos_url": "https://github.com/api/v3/users/octocat/repos",
        "events_url": "https://github.com/api/v3/users/octocat/events{/privacy}",
        "received_events_url": "https://github.com/api/v3/users/octocat/received_events",
        "type": "User",
        "site_admin": False
    },
    "committer": {
        "login": "octocat",
        "id": 583231,
        "node_id": "MDQ6VXNlcjU4MzIzMQ==",
        "avatar_url": "https://avatars3.githubusercontent.com/u/583231?v=4",
        "gravatar_id": "",
        "url": "https://github.com/api/v3/users/octocat",
        "html_url": "https://github.com/octocat",
        "followers_url": "https://github.com/api/v3/users/octocat/followers",
        "following_url": "https://github.com/api/v3/users/octocat/following{/other_user}",
        "gists_url": "https://github.com/api/v3/users/octocat/gists{/gist_id}",
        "starred_url": "https://github.com/api/v3/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "https://github.com/api/v3/users/octocat/subscriptions",
        "organizations_url": "https://github.com/api/v3/users/octocat/orgs",
        "repos_url": "https://github.com/api/v3/users/octocat/repos",
        "events_url": "https://github.com/api/v3/users/octocat/events{/privacy}",
        "received_events_url": "https://github.com/api/v3/users/octocat/received_events",
        "type": "User",
        "site_admin": False
    },
    "parents": [
        {
            "sha": "553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
            "url": "https://github.com/api/v3/repos/octocat/Hello-World/commits/553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
            "html_url": "https://github.com/octocat/Hello-World/commit/553c2077f0edc3d5dc5d17262f6aa498e69d6f8e"
        },
        {
            "sha": "762941318ee16e59dabbacb1b4049eec22f0d303",
            "url": "https://github.com/api/v3/repos/octocat/Hello-World/commits/762941318ee16e59dabbacb1b4049eec22f0d303",
            "html_url": "https://github.com/octocat/Hello-World/commit/762941318ee16e59dabbacb1b4049eec22f0d303"
        }
    ],
    "stats": {
        "total": 2,
        "additions": 1,
        "deletions": 1
    },
    "files": [
        {
            "sha": "980a0d5f19a64b4b30a87d4206aade58726b60e3",
            "filename": "README",
            "status": "modified",
            "additions": 1,
            "deletions": 1,
            "changes": 2,
            "blob_url": "https://github.com/octocat/Hello-World/blob/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d/README",
            "raw_url": "https://github.com/octocat/Hello-World/raw/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d/README",
            "contents_url": "https://github.com/api/v3/repos/octocat/Hello-World/contents/README?ref=7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
            "patch": "@@ -1 +1 @@\n-Hello World!\n\\ No newline at end of file\n+Hello World!"
        }
    ]
}

GITHUB_BRANCH_PAYLOAD = [
    {
        "name": "master",
        "commit": {
            "sha": "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
            "url": "https://github.com/api/v3/repos/octocat/Hello-World/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d"
        }
    }
]
