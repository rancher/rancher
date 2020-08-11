# -*- coding: utf-8 -*-
from .baseapi import BaseAPI, GET, POST, PUT, DELETE


class StickySesions(object):
    """
    An object holding information on a LoadBalancer's sticky sessions settings.

    Args:
        type (str): The type of sticky sessions used. Can be "cookies" or
            "none"
        cookie_name (str, optional): The name used for the client cookie when
            using cookies for sticky session
        cookie_ttl_seconds (int, optional): The number of seconds until the
            cookie expires
    """
    def __init__(self, type='none', cookie_name='', cookie_ttl_seconds=None):
        self.type = type
        if type is 'cookies':
            self.cookie_name = 'DO-LB'
            self.cookie_ttl_seconds = 300
        self.cookie_name = cookie_name
        self.cookie_ttl_seconds = cookie_ttl_seconds


class ForwardingRule(object):
    """
    An object holding information about a LoadBalancer forwarding rule setting.

    Args:
        entry_protocol (str): The protocol used for traffic to a LoadBalancer.
            The possible values are: "http", "https", or "tcp"
        entry_port (int): The port the LoadBalancer instance will listen on
        target_protocol (str): The protocol used for traffic from a
            LoadBalancer to the backend Droplets. The possible values are:
            "http", "https", or "tcp"
        target_port (int): The port on the backend Droplets on which the
            LoadBalancer will send traffic
        certificate_id (str, optional): The ID of the TLS certificate used for
            SSL termination if enabled
        tls_passthrough (bool, optional): A boolean indicating if SSL encrypted
            traffic will be passed through to the backend Droplets
    """
    def __init__(self, entry_protocol=None, entry_port=None,
                 target_protocol=None, target_port=None, certificate_id="",
                 tls_passthrough=False):
        self.entry_protocol = entry_protocol
        self.entry_port = entry_port
        self.target_protocol = target_protocol
        self.target_port = target_port
        self.certificate_id = certificate_id
        self.tls_passthrough = tls_passthrough


class HealthCheck(object):
    """
    An object holding information about a LoadBalancer health check settings.

    Args:
        protocol (str): The protocol used for health checks. The possible
            values are "http" or "tcp".
        port (int): The port on the backend Droplets for heath checks
        path (str): The path to send a health check request to
        check_interval_seconds (int): The number of seconds between between two
            consecutive health checks
        response_timeout_seconds (int): The number of seconds the Load Balancer
            instance will wait for a response until marking a check as failed
        healthy_threshold (int): The number of times a health check must fail
            for a backend Droplet to be removed from the pool
        unhealthy_threshold (int): The number of times a health check must pass
            for a backend Droplet to be re-added to the pool
    """
    def __init__(self, protocol='http', port=80, path='/',
                 check_interval_seconds=10, response_timeout_seconds=5,
                 healthy_threshold=5, unhealthy_threshold=3):
        self.protocol = protocol
        self.port = port
        self.path = path
        self.check_interval_seconds = check_interval_seconds
        self.response_timeout_seconds = response_timeout_seconds
        self.healthy_threshold = healthy_threshold
        self.unhealthy_threshold = unhealthy_threshold


class LoadBalancer(BaseAPI):
    """
    An object representing an DigitalOcean Load Balancer.

    Attributes accepted at creation time:

    Args:
        name (str): The Load Balancer's name
        region (str): The slug identifier for a DigitalOcean region
        algorithm (str, optional): The load balancing algorithm to be
            used. Currently, it must be either "round_robin" or
            "least_connections"
        forwarding_rules (obj:`list`): A list of `ForwrdingRules` objects
        health_check (obj, optional): A `HealthCheck` object
        sticky_sessions (obj, optional): A `StickySessions` object
        redirect_http_to_https (bool, optional): A boolean indicating
            whether HTTP requests to the Load Balancer should be
            redirected to HTTPS
        droplet_ids (obj:`list` of `int`): A list of IDs representing
            Droplets to be added to the Load Balancer (mutually
            exclusive with 'tag')
        tag (str): A string representing a DigitalOcean Droplet tag
            (mutually exclusive with 'droplet_ids')

   Attributes returned by API:
        name (str): The Load Balancer's name
        id (str): An unique identifier for a LoadBalancer
        ip (str): Public IP address for a LoadBalancer
        region (str): The slug identifier for a DigitalOcean region
        algorithm (str, optional): The load balancing algorithm to be
            used. Currently, it must be either "round_robin" or
            "least_connections"
        forwarding_rules (obj:`list`): A list of `ForwrdingRules` objects
        health_check (obj, optional): A `HealthCheck` object
        sticky_sessions (obj, optional): A `StickySessions` object
        redirect_http_to_https (bool, optional): A boolean indicating
            whether HTTP requests to the Load Balancer should be
            redirected to HTTPS
        droplet_ids (obj:`list` of `int`): A list of IDs representing
            Droplets to be added to the Load Balancer
        tag (str): A string representing a DigitalOcean Droplet tag
        status (string): An indication the current state of the LoadBalancer
        created_at (str): The date and time when the LoadBalancer was created
    """
    def __init__(self, *args, **kwargs):
        self.id = None
        self.name = None
        self.region = None
        self.algorithm = None
        self.forwarding_rules = []
        self.health_check = None
        self.sticky_sessions = None
        self.redirect_http_to_https = False
        self.droplet_ids = []
        self.tag = None
        self.status = None
        self.created_at = None

        super(LoadBalancer, self).__init__(*args, **kwargs)

    @classmethod
    def get_object(cls, api_token, id):
        """
        Class method that will return a LoadBalancer object by its ID.

        Args:
            api_token (str): DigitalOcean API token
            id (str): Load Balancer ID
        """
        load_balancer = cls(token=api_token, id=id)
        load_balancer.load()
        return load_balancer

    def load(self):
        """
        Loads updated attributues for a LoadBalancer object.

        Requires self.id to be set.
        """
        data = self.get_data('load_balancers/%s' % self.id, type=GET)
        load_balancer = data['load_balancer']

        # Setting the attribute values
        for attr in load_balancer.keys():
            if attr == 'health_check':
                health_check = HealthCheck(**load_balancer['health_check'])
                setattr(self, attr, health_check)
            elif attr == 'sticky_sessions':
                sticky_ses = StickySesions(**load_balancer['sticky_sessions'])
                setattr(self, attr, sticky_ses)
            elif attr == 'forwarding_rules':
                rules = list()
                for rule in load_balancer['forwarding_rules']:
                    rules.append(ForwardingRule(**rule))
                setattr(self, attr, rules)
            else:
                setattr(self, attr, load_balancer[attr])

        return self

    def create(self, *args, **kwargs):
        """
        Creates a new LoadBalancer.

        Note: Every argument and parameter given to this method will be
        assigned to the object.

        Args:
            name (str): The Load Balancer's name
            region (str): The slug identifier for a DigitalOcean region
            algorithm (str, optional): The load balancing algorithm to be
                used. Currently, it must be either "round_robin" or
                "least_connections"
            forwarding_rules (obj:`list`): A list of `ForwrdingRules` objects
            health_check (obj, optional): A `HealthCheck` object
            sticky_sessions (obj, optional): A `StickySessions` object
            redirect_http_to_https (bool, optional): A boolean indicating
                whether HTTP requests to the Load Balancer should be
                redirected to HTTPS
            droplet_ids (obj:`list` of `int`): A list of IDs representing
                Droplets to be added to the Load Balancer (mutually
                exclusive with 'tag')
            tag (str): A string representing a DigitalOcean Droplet tag
                (mutually exclusive with 'droplet_ids')
        """
        rules_dict = [rule.__dict__ for rule in self.forwarding_rules]

        params = {'name': self.name, 'region': self.region,
                  'forwarding_rules': rules_dict,
                  'redirect_http_to_https': self.redirect_http_to_https}

        if self.droplet_ids and self.tag:
            raise ValueError('droplet_ids and tag are mutually exclusive args')
        elif self.tag:
            params['tag'] = self.tag
        else:
            params['droplet_ids'] = self.droplet_ids

        if self.algorithm:
            params['algorithm'] = self.algorithm
        if self.health_check:
            params['health_check'] = self.health_check.__dict__
        if self.sticky_sessions:
            params['sticky_sessions'] = self.sticky_sessions.__dict__

        data = self.get_data('load_balancers/', type=POST, params=params)

        if data:
            self.id = data['load_balancer']['id']
            self.ip = data['load_balancer']['ip']
            self.algorithm = data['load_balancer']['algorithm']
            self.health_check = HealthCheck(
                **data['load_balancer']['health_check'])
            self.sticky_sessions = StickySesions(
                **data['load_balancer']['sticky_sessions'])
            self.droplet_ids = data['load_balancer']['droplet_ids']
            self.status = data['load_balancer']['status']
            self.created_at = data['load_balancer']['created_at']

        return self

    def save(self):
        """
        Save the LoadBalancer
        """
        forwarding_rules = [rule.__dict__ for rule in self.forwarding_rules]

        data = {
            'name': self.name,
            'region': self.region['slug'],
            'forwarding_rules': forwarding_rules,
            'redirect_http_to_https': self.redirect_http_to_https
        }

        if self.tag:
            data['tag'] = self.tag
        else:
            data['droplet_ids'] = self.droplet_ids

        if self.algorithm:
            data["algorithm"] = self.algorithm
        if self.health_check:
            data['health_check'] = self.health_check.__dict__
        if self.sticky_sessions:
            data['sticky_sessions'] = self.sticky_sessions.__dict__

        return self.get_data("load_balancers/%s/" % self.id,
                             type=PUT,
                             params=data)

    def destroy(self):
        """
        Destroy the LoadBalancer
        """
        return self.get_data('load_balancers/%s/' % self.id, type=DELETE)

    def add_droplets(self, droplet_ids):
        """
        Assign a LoadBalancer to a Droplet.

        Args:
            droplet_ids (obj:`list` of `int`): A list of Droplet IDs
        """
        return self.get_data(
            "load_balancers/%s/droplets/" % self.id,
            type=POST,
            params={"droplet_ids": droplet_ids}
        )

    def remove_droplets(self, droplet_ids):
        """
        Unassign a LoadBalancer.

        Args:
            droplet_ids (obj:`list` of `int`): A list of Droplet IDs
        """
        return self.get_data(
            "load_balancers/%s/droplets/" % self.id,
            type=DELETE,
            params={"droplet_ids": droplet_ids}
        )

    def add_forwarding_rules(self, forwarding_rules):
        """
        Adds new forwarding rules to a LoadBalancer.

        Args:
            forwarding_rules (obj:`list`): A list of `ForwrdingRules` objects
        """
        rules_dict = [rule.__dict__ for rule in forwarding_rules]

        return self.get_data(
            "load_balancers/%s/forwarding_rules/" % self.id,
            type=POST,
            params={"forwarding_rules": rules_dict}
        )

    def remove_forwarding_rules(self, forwarding_rules):
        """
        Removes existing forwarding rules from a LoadBalancer.

        Args:
            forwarding_rules (obj:`list`): A list of `ForwrdingRules` objects
        """
        rules_dict = [rule.__dict__ for rule in forwarding_rules]

        return self.get_data(
            "load_balancers/%s/forwarding_rules/" % self.id,
            type=DELETE,
            params={"forwarding_rules": rules_dict}
        )

    def __str__(self):
        return "%s" % (self.id)
