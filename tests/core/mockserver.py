# flake8: noqa
import requests

from flask import jsonify
from threading import Thread

MOCK_GITHUB_PORT = 4016
MOCK_GITLAB_PORT = 4017


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

    def __init__(self, port=4016):
        super().__init__(port)
        self.add_endpoints()


class MockGitlab(MockServer):

    def api_user(self):
        return jsonify(GITLAB_USER_PAYLOAD)

    def api_repos(self):
        return jsonify(GITLAB_REPOS_PAYLOAD)

    def api_file_content(self):
        return jsonify(GITLAB_FILE_CONTENT_PAYLOAD)

    def api_commit(self):
        return jsonify(GITLAB_COMMIT_PAYLOAD)

    def api_branch(self):
        return jsonify(GITLAB_BRANCH_PAYLOAD)

    def api_access_token(self):
        return jsonify({'access_token': 'test_token', 'token_type': 'bearer'})

    def add_endpoints(self):
        self.app.add_url_rule("/oauth/token", view_func=self.api_access_token,
                              methods=('POST',))
        self.app.add_url_rule("/api/v4/user", view_func=self.api_user)
        self.app.add_url_rule("/api/v4/projects", view_func=self.api_repos)
        self.app.add_url_rule(
            "/api/v4/projects/brightbox/puppet/repository/files/.rancher-pipeline.yml",
            view_func=self.api_file_content)
        self.app.add_url_rule(
            "/api/v4/projects/brightbox/puppet/repository/commits",
            view_func=self.api_commit)
        self.app.add_url_rule(
            "/api/v4/projects/brightbox/puppet/repository/branches",
            view_func=self.api_branch)
        pass

    def __init__(self, port=MOCK_GITLAB_PORT):
        super().__init__(port)
        self.add_endpoints()


GITHUB_USER_PAYLOAD = {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "http://localhost:4016/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "http://localhost:4016/api/v3/users/octocat",
    "html_url": "http://localhost:4016/octocat",
    "followers_url": "http://localhost:4016/api/v3/users/octocat/followers",
    "following_url": "http://localhost:4016/api/v3/users/octocat/following{/other_user}",
    "gists_url": "http://localhost:4016/api/v3/users/octocat/gists{/gist_id}",
    "starred_url": "http://localhost:4016/api/v3/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "http://localhost:4016/api/v3/users/octocat/subscriptions",
    "organizations_url": "http://localhost:4016/api/v3/users/octocat/orgs",
    "repos_url": "http://localhost:4016/api/v3/users/octocat/repos",
    "events_url": "http://localhost:4016/api/v3/users/octocat/events{/privacy}",
    "received_events_url": "http://localhost:4016/api/v3/users/octocat/received_events",
    "type": "User",
    "site_admin": False,
    "name": "monalisa octocat",
    "company": "GitHub",
    "blog": "http://localhost:4016/blog",
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
            "avatar_url": "http://localhost:4016/images/error/octocat_happy.gif",
            "gravatar_id": "",
            "url": "http://localhost:4016/api/v3/users/octocat",
            "html_url": "http://localhost:4016/octocat",
            "followers_url": "http://localhost:4016/api/v3/users/octocat/followers",
            "following_url": "http://localhost:4016/api/v3/users/octocat/following{/other_user}",
            "gists_url": "http://localhost:4016/api/v3/users/octocat/gists{/gist_id}",
            "starred_url": "http://localhost:4016/api/v3/users/octocat/starred{/owner}{/repo}",
            "subscriptions_url": "http://localhost:4016/api/v3/users/octocat/subscriptions",
            "organizations_url": "http://localhost:4016/api/v3/users/octocat/orgs",
            "repos_url": "http://localhost:4016/api/v3/users/octocat/repos",
            "events_url": "http://localhost:4016/api/v3/users/octocat/events{/privacy}",
            "received_events_url": "http://localhost:4016/api/v3/users/octocat/received_events",
            "type": "User",
            "site_admin": False
        },
        "private": False,
        "html_url": "http://localhost:4016/octocat/Hello-World",
        "description": "This your first repo!",
        "fork": False,
        "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World",
        "archive_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/{archive_format}{/ref}",
        "assignees_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/assignees{/user}",
        "blobs_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/blobs{/sha}",
        "branches_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/branches{/branch}",
        "collaborators_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/collaborators{/collaborator}",
        "comments_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/comments{/number}",
        "commits_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/commits{/sha}",
        "compare_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/compare/{base}...{head}",
        "contents_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/contents/{+path}",
        "contributors_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/contributors",
        "deployments_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/deployments",
        "downloads_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/downloads",
        "events_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/events",
        "forks_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/forks",
        "git_commits_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/commits{/sha}",
        "git_refs_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/refs{/sha}",
        "git_tags_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/tags{/sha}",
        "git_url": "git:localhost:4016/octocat/Hello-World.git",
        "issue_comment_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/issues/comments{/number}",
        "issue_events_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/issues/events{/number}",
        "issues_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/issues{/number}",
        "keys_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/keys{/key_id}",
        "labels_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/labels{/name}",
        "languages_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/languages",
        "merges_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/merges",
        "milestones_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/milestones{/number}",
        "notifications_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/notifications{?since,all,participating}",
        "pulls_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/pulls{/number}",
        "releases_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/releases{/id}",
        "ssh_url": "git@localhost:4016:octocat/Hello-World.git",
        "stargazers_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/stargazers",
        "statuses_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/statuses/{sha}",
        "subscribers_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/subscribers",
        "subscription_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/subscription",
        "tags_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/tags",
        "teams_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/teams",
        "trees_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/trees{/sha}",
        "clone_url": "http://localhost:4016/octocat/Hello-World.git",
        "mirror_url": "git:git.example.com/octocat/Hello-World",
        "hooks_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/hooks",
        "svn_url": "https://svn.localhost:4016/octocat/Hello-World",
        "homepage": "http://localhost:4016",
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
            "url": "http://localhost:4016/api/v3/licenses/mit",
            "node_id": "MDc6TGljZW5zZW1pdA=="
        }
    }
]

GITHUB_FILE_CONTENT_PAYLOAD = {
    "name": ".rancher-pipeline.yml",
    "path": ".rancher-pipeline.yml",
    "sha": "e849c8954bad15cdccd309d3d434b7580e3246ce",
    "size": 881,
    "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/contents/.rancher-pipeline.yml?ref=master",
    "html_url": "http://localhost:4016/octocat/Hello-World/blob/master/.rancher-pipeline.yml",
    "git_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/blobs/e849c8954bad15cdccd309d3d434b7580e3246ce",
    "type": "file",
    "content": "c3RhZ2VzOgotIG5hbWU6IENvZGVjZXB0aW9uIHRlc3QKICBzdGVwczoKICAt\nIHJ1blNjcmlwdENvbmZpZzoKICAgICAgaW1hZ2U6IHBocDo3LjIKICAgICAg\nc2hlbGxTY3JpcHQ6IHwtCiAgICAgICAgYXB0LWdldCB1cGRhdGUKICAgICAg\nICBhcHQtZ2V0IGluc3RhbGwgLXkgLS1uby1pbnN0YWxsLXJlY29tbWVuZHMg\nZ2l0IHppcCBsaWJzcWxpdGUzLWRldiB6bGliMWctZGV2CiAgICAgICAgZG9j\na2VyLXBocC1leHQtaW5zdGFsbCB6aXAKICAgICAgICBjdXJsIC0tc2lsZW50\nIC0tc2hvdy1lcnJvciBodHRwczovL2dldGNvbXBvc2VyLm9yZy9pbnN0YWxs\nZXIgfCBwaHAKICAgICAgICAuL2NvbXBvc2VyLnBoYXIgaW5zdGFsbCAtbiAt\nLXByZWZlci1kaXN0CiAgICAgICAgdG91Y2ggc3RvcmFnZS90ZXN0aW5nLnNx\nbGl0ZSBzdG9yYWdlL2RhdGFiYXNlLnNxbGl0ZQogICAgICAgIGNwIC5lbnYu\ndGVzdGluZyAuZW52CiAgICAgICAgcGhwIGFydGlzYW4gbWlncmF0ZQogICAg\nICAgIHBocCBhcnRpc2FuIG1pZ3JhdGUgLS1lbnY9dGVzdGluZyAtLWRhdGFi\nYXNlPXNxbGl0ZV90ZXN0aW5nIC0tZm9yY2UKICAgICAgICAuL3ZlbmRvci9i\naW4vY29kZWNlcHQgYnVpbGQKICAgICAgICAuL3ZlbmRvci9iaW4vY29kZWNl\ncHQgcnVuCi0gbmFtZTogUHVibGlzaCBpbWFnZQogIHN0ZXBzOgogIC0gcHVi\nbGlzaEltYWdlQ29uZmlnOgogICAgICBkb2NrZXJmaWxlUGF0aDogLi9Eb2Nr\nZXJmaWxlCiAgICAgIGJ1aWxkQ29udGV4dDogLgogICAgICB0YWc6IHBocC1l\neGFtcGxlOiR7Q0lDRF9FWEVDVVRJT05fU0VRVUVOQ0V9Ci0gbmFtZTogRGVw\nbG95CiAgc3RlcHM6CiAgLSBhcHBseVlhbWxDb25maWc6CiAgICAgIHBhdGg6\nIC4vZGVwbG95L2RlcGxveW1lbnQueWFtbAo=\n",
    "encoding": "base64",
    "_links": {
        "self": "http://localhost:4016/api/v3/repos/octocat/Hello-World/contents/.rancher-pipeline.yml?ref=master",
        "git": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/blobs/e849c8954bad15cdccd309d3d434b7580e3246ce",
        "html": "http://localhost:4016/octocat/Hello-World/blob/master/.rancher-pipeline.yml"
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
            "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/trees/b4eecafa9be2f2006ce1b709d6857b07069b4608"
        },
        "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/git/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
        "comment_count": 55,
        "verification": {
            "verified": False,
            "reason": "unsigned",
            "signature": None,
            "payload": None
        }
    },
    "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
    "html_url": "http://localhost:4016/octocat/Hello-World/commit/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
    "comments_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d/comments",
    "author": {
        "login": "octocat",
        "id": 583231,
        "node_id": "MDQ6VXNlcjU4MzIzMQ==",
        "avatar_url": "https://avatars3.githubusercontent.com/u/583231?v=4",
        "gravatar_id": "",
        "url": "http://localhost:4016/api/v3/users/octocat",
        "html_url": "http://localhost:4016/octocat",
        "followers_url": "http://localhost:4016/api/v3/users/octocat/followers",
        "following_url": "http://localhost:4016/api/v3/users/octocat/following{/other_user}",
        "gists_url": "http://localhost:4016/api/v3/users/octocat/gists{/gist_id}",
        "starred_url": "http://localhost:4016/api/v3/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "http://localhost:4016/api/v3/users/octocat/subscriptions",
        "organizations_url": "http://localhost:4016/api/v3/users/octocat/orgs",
        "repos_url": "http://localhost:4016/api/v3/users/octocat/repos",
        "events_url": "http://localhost:4016/api/v3/users/octocat/events{/privacy}",
        "received_events_url": "http://localhost:4016/api/v3/users/octocat/received_events",
        "type": "User",
        "site_admin": False
    },
    "committer": {
        "login": "octocat",
        "id": 583231,
        "node_id": "MDQ6VXNlcjU4MzIzMQ==",
        "avatar_url": "https://avatars3.githubusercontent.com/u/583231?v=4",
        "gravatar_id": "",
        "url": "http://localhost:4016/api/v3/users/octocat",
        "html_url": "http://localhost:4016/octocat",
        "followers_url": "http://localhost:4016/api/v3/users/octocat/followers",
        "following_url": "http://localhost:4016/api/v3/users/octocat/following{/other_user}",
        "gists_url": "http://localhost:4016/api/v3/users/octocat/gists{/gist_id}",
        "starred_url": "http://localhost:4016/api/v3/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "http://localhost:4016/api/v3/users/octocat/subscriptions",
        "organizations_url": "http://localhost:4016/api/v3/users/octocat/orgs",
        "repos_url": "http://localhost:4016/api/v3/users/octocat/repos",
        "events_url": "http://localhost:4016/api/v3/users/octocat/events{/privacy}",
        "received_events_url": "http://localhost:4016/api/v3/users/octocat/received_events",
        "type": "User",
        "site_admin": False
    },
    "parents": [
        {
            "sha": "553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
            "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/commits/553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
            "html_url": "http://localhost:4016/octocat/Hello-World/commit/553c2077f0edc3d5dc5d17262f6aa498e69d6f8e"
        },
        {
            "sha": "762941318ee16e59dabbacb1b4049eec22f0d303",
            "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/commits/762941318ee16e59dabbacb1b4049eec22f0d303",
            "html_url": "http://localhost:4016/octocat/Hello-World/commit/762941318ee16e59dabbacb1b4049eec22f0d303"
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
            "blob_url": "http://localhost:4016/octocat/Hello-World/blob/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d/README",
            "raw_url": "http://localhost:4016/octocat/Hello-World/raw/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d/README",
            "contents_url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/contents/README?ref=7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
            "patch": "@@ -1 +1 @@\n-Hello World!\n\\ No newline at end of file\n+Hello World!"
        }
    ]
}

GITHUB_BRANCH_PAYLOAD = [
    {
        "name": "master",
        "commit": {
            "sha": "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
            "url": "http://localhost:4016/api/v3/repos/octocat/Hello-World/commits/7fd1a60b01f91b314f59955a4e4d4e80d8edf11d"
        }
    }
]

GITLAB_USER_PAYLOAD = {
    "id": 1,
    "username": "john_smith",
    "email": "john@example.com",
    "name": "John Smith",
    "state": "active",
    "avatar_url": "http://localhost:4017/uploads/user/avatar/1/index.jpg",
    "web_url": "http://localhost:4017/john_smith",
    "created_at": "2012-05-23T08:00:58Z",
    "bio": None,
    "location": None,
    "skype": "",
    "linkedin": "",
    "twitter": "",
    "website_url": "",
    "organization": "",
    "last_sign_in_at": "2012-06-01T11:41:01Z",
    "confirmed_at": "2012-05-23T09:05:22Z",
    "theme_id": 1,
    "last_activity_on": "2012-05-23",
    "color_scheme_id": 2,
    "projects_limit": 100,
    "current_sign_in_at": "2012-06-02T06:36:55Z",
    "identities": [
        {"provider": "github", "extern_uid": "2435223452345"},
        {"provider": "bitbucket", "extern_uid": "john_smith"},
        {"provider": "google_oauth2",
         "extern_uid": "8776128412476123468721346"}
    ],
    "can_create_group": True,
    "can_create_project": True,
    "two_factor_enabled": True,
    "external": False,
    "private_profile": False
}

GITLAB_REPOS_PAYLOAD = [
    {
        "id": 4,
        "description": None,
        "default_branch": "master",
        "visibility": "private",
        "ssh_url_to_repo": "git@example.com:diaspora/diaspora-client.git",
        "http_url_to_repo": "http://localhost:4017/diaspora/diaspora-client.git",
        "web_url": "http://localhost:4017/diaspora/diaspora-client",
        "readme_url": "http://localhost:4017/diaspora/diaspora-client/blob/master/README.md",
        "tag_list": [
            "example",
            "disapora client"
        ],
        "owner": {
            "id": 3,
            "name": "Diaspora",
            "created_at": "2013-09-30T13:46:02Z"
        },
        "name": "Diaspora Client",
        "name_with_namespace": "Diaspora / Diaspora Client",
        "path": "diaspora-client",
        "path_with_namespace": "diaspora/diaspora-client",
        "issues_enabled": True,
        "open_issues_count": 1,
        "merge_requests_enabled": True,
        "jobs_enabled": True,
        "wiki_enabled": True,
        "snippets_enabled": False,
        "resolve_outdated_diff_discussions": False,
        "container_registry_enabled": False,
        "created_at": "2013-09-30T13:46:02Z",
        "last_activity_at": "2013-09-30T13:46:02Z",
        "creator_id": 3,
        "namespace": {
            "id": 3,
            "name": "Diaspora",
            "path": "diaspora",
            "kind": "group",
            "full_path": "diaspora"
        },
        "import_status": "none",
        "archived": False,
        "avatar_url": "http://localhost:4017/uploads/project/avatar/4/uploads/avatar.png",
        "shared_runners_enabled": True,
        "forks_count": 0,
        "star_count": 0,
        "runners_token": "b8547b1dc37721d05889db52fa2f02",
        "public_jobs": True,
        "shared_with_groups": [],
        "only_allow_merge_if_pipeline_succeeds": False,
        "only_allow_merge_if_all_discussions_are_resolved": False,
        "request_access_enabled": False,
        "merge_method": "merge",
        "statistics": {
            "commit_count": 37,
            "storage_size": 1038090,
            "repository_size": 1038090,
            "lfs_objects_size": 0,
            "job_artifacts_size": 0
        },
        "_links": {
            "self": "http://localhost:4017/api/v4/projects",
            "issues": "http://localhost:4017/api/v4/projects/1/issues",
            "merge_requests": "http://localhost:4017/api/v4/projects/1/merge_requests",
            "repo_branches": "http://localhost:4017/api/v4/projects/1/repository_branches",
            "labels": "http://localhost:4017/api/v4/projects/1/labels",
            "events": "http://localhost:4017/api/v4/projects/1/events",
            "members": "http://localhost:4017/api/v4/projects/1/members"
        }
    },
    {
        "id": 6,
        "description": None,
        "default_branch": "master",
        "visibility": "private",
        "ssh_url_to_repo": "git@example.com:brightbox/puppet.git",
        "http_url_to_repo": "http://localhost:4017/brightbox/puppet.git",
        "web_url": "http://localhost:4017/brightbox/puppet",
        "readme_url": "http://localhost:4017/brightbox/puppet/blob/master/README.md",
        "tag_list": [
            "example",
            "puppet"
        ],
        "owner": {
            "id": 4,
            "name": "Brightbox",
            "created_at": "2013-09-30T13:46:02Z"
        },
        "name": "Puppet",
        "name_with_namespace": "Brightbox / Puppet",
        "path": "puppet",
        "path_with_namespace": "brightbox/puppet",
        "issues_enabled": True,
        "open_issues_count": 1,
        "merge_requests_enabled": True,
        "jobs_enabled": True,
        "wiki_enabled": True,
        "snippets_enabled": False,
        "resolve_outdated_diff_discussions": False,
        "container_registry_enabled": False,
        "created_at": "2013-09-30T13:46:02Z",
        "last_activity_at": "2013-09-30T13:46:02Z",
        "creator_id": 3,
        "namespace": {
            "id": 4,
            "name": "Brightbox",
            "path": "brightbox",
            "kind": "group",
            "full_path": "brightbox"
        },
        "import_status": "none",
        "import_error": None,
        "permissions": {
            "project_access": {
                "access_level": 10,
                "notification_level": 3
            },
            "group_access": {
                "access_level": 50,
                "notification_level": 3
            }
        },
        "archived": False,
        "avatar_url": None,
        "shared_runners_enabled": True,
        "forks_count": 0,
        "star_count": 0,
        "runners_token": "b8547b1dc37721d05889db52fa2f02",
        "public_jobs": True,
        "shared_with_groups": [],
        "only_allow_merge_if_pipeline_succeeds": False,
        "only_allow_merge_if_all_discussions_are_resolved": False,
        "request_access_enabled": False,
        "merge_method": "merge",
        "statistics": {
            "commit_count": 12,
            "storage_size": 2066080,
            "repository_size": 2066080,
            "lfs_objects_size": 0,
            "job_artifacts_size": 0
        },
        "_links": {
            "self": "http://localhost:4017/api/v4/projects",
            "issues": "http://localhost:4017/api/v4/projects/1/issues",
            "merge_requests": "http://localhost:4017/api/v4/projects/1/merge_requests",
            "repo_branches": "http://localhost:4017/api/v4/projects/1/repository_branches",
            "labels": "http://localhost:4017/api/v4/projects/1/labels",
            "events": "http://localhost:4017/api/v4/projects/1/events",
            "members": "http://localhost:4017/api/v4/projects/1/members"
        }
    }
]

GITLAB_FILE_CONTENT_PAYLOAD = {
    "file_name": ".rancher-pipeline.yml",
    "file_path": ".rancher-pipeline.yml",
    "size": 161,
    "encoding": "base64",
    "content_sha256": "013322d3670a03a687efc52cf63fa3f64011dcc2ce980faf82fa6a284aad812e",
    "ref": "master",
    "blob_id": "8e98616f0cc5e238d729c84814750738ff4ad9cd",
    "commit_id": "551da3bc9b7c78f8566e59a20722592f1bcbdab6",
    "last_commit_id": "551da3bc9b7c78f8566e59a20722592f1bcbdab6",
    "content": "c3RhZ2VzOgotIG5hbWU6IGJ1aWxkCiAgc3RlcHM6CiAgLSBydW5TY3JpcHRDb25maWc6CiAgICAgIGltYWdlOiBidXN5Ym94CiAgICAgIHNoZWxsU2NyaXB0OiB8LQogICAgICAgIGVjaG8gaGlpbm1hc3RlcgogICAgICAgIGVjaG8gaW5naXRsYWIyCiAgICAgICAgZWNobyBkb25lYQo="
}

GITLAB_COMMIT_PAYLOAD = [
    {
        "id": "ed899a2f4b50b4370feeea94676502b42383c746",
        "short_id": "ed899a2f4b5",
        "title": "Replace sanitize with escape once",
        "author_name": "Dmitriy Zaporozhets",
        "author_email": "dzaporozhets@sphereconsultinginc.com",
        "authored_date": "2012-09-20T11:50:22+03:00",
        "committer_name": "Administrator",
        "committer_email": "admin@example.com",
        "committed_date": "2012-09-20T11:50:22+03:00",
        "created_at": "2012-09-20T11:50:22+03:00",
        "message": "Replace sanitize with escape once",
        "parent_ids": [
            "6104942438c14ec7bd21c6cd5bd995272b3faff6"
        ]
    },
    {
        "id": "6104942438c14ec7bd21c6cd5bd995272b3faff6",
        "short_id": "6104942438c",
        "title": "Sanitize for network graph",
        "author_name": "randx",
        "author_email": "dmitriy.zaporozhets@gmail.com",
        "committer_name": "Dmitriy",
        "committer_email": "dmitriy.zaporozhets@gmail.com",
        "created_at": "2012-09-20T09:06:12+03:00",
        "message": "Sanitize for network graph",
        "parent_ids": [
            "ae1d9fb46aa2b07ee9836d49862ec4e2c46fbbba"
        ]
    }
]

GITLAB_BRANCH_PAYLOAD = [
    {
        "name": "master",
        "merged": False,
        "protected": True,
        "developers_can_push": False,
        "developers_can_merge": False,
        "can_push": True,
        "commit": {
            "author_email": "john@example.com",
            "author_name": "John Smith",
            "authored_date": "2012-06-27T05:51:39-07:00",
            "committed_date": "2012-06-28T03:44:20-07:00",
            "committer_email": "john@example.com",
            "committer_name": "John Smith",
            "id": "7b5c3cc8be40ee161ae89a06bba6229da1032a0c",
            "short_id": "7b5c3cc",
            "title": "add projects API",
            "message": "add projects API",
            "parent_ids": [
                "4ad91d3c1144c406e50c7b33bae684bd6837faf8"
            ]
        }
    }
]
