# -*- coding: utf-8 -*-
try:
    from urlparse import urlparse, parse_qs
except ImportError:
    from urllib.parse import urlparse, parse_qs                 # noqa

from .baseapi import BaseAPI
from .Account import Account
from .Action import Action
from .Certificate import Certificate
from .Domain import Domain
from .Droplet import Droplet
from .FloatingIP import FloatingIP
from .Firewall import Firewall, InboundRule, OutboundRule
from .Image import Image
from .LoadBalancer import LoadBalancer
from .LoadBalancer import StickySesions, HealthCheck, ForwardingRule
from .Region import Region
from .SSHKey import SSHKey
from .Size import Size
from .Snapshot import Snapshot
from .Tag import Tag
from .Volume import Volume


class Manager(BaseAPI):
    def __init__(self, *args, **kwargs):
        super(Manager, self).__init__(*args, **kwargs)

    def get_account(self):
        """
            Returns an Account object.
        """
        return Account.get_object(api_token=self.token)

    def get_all_regions(self):
        """
            This function returns a list of Region object.
        """
        data = self.get_data("regions/")
        regions = list()
        for jsoned in data['regions']:
            region = Region(**jsoned)
            region.token = self.token
            regions.append(region)
        return regions

    def get_all_droplets(self, tag_name=None):
        """
            This function returns a list of Droplet object.
        """
        params = dict()
        if tag_name:
            params["tag_name"] = tag_name

        data = self.get_data("droplets/", params=params)

        droplets = list()
        for jsoned in data['droplets']:
            droplet = Droplet(**jsoned)
            droplet.token = self.token

            for net in droplet.networks['v4']:
                if net['type'] == 'private':
                    droplet.private_ip_address = net['ip_address']
                if net['type'] == 'public':
                    droplet.ip_address = net['ip_address']
            if droplet.networks['v6']:
                droplet.ip_v6_address = droplet.networks['v6'][0]['ip_address']

            if "backups" in droplet.features:
                droplet.backups = True
            else:
                droplet.backups = False
            if "ipv6" in droplet.features:
                droplet.ipv6 = True
            else:
                droplet.ipv6 = False
            if "private_networking" in droplet.features:
                droplet.private_networking = True
            else:
                droplet.private_networking = False

            droplets.append(droplet)

        return droplets

    def get_droplet(self, droplet_id):
        """
            Return a Droplet by its ID.
        """
        return Droplet.get_object(api_token=self.token, droplet_id=droplet_id)

    def get_all_sizes(self):
        """
            This function returns a list of Size object.
        """
        data = self.get_data("sizes/")
        sizes = list()
        for jsoned in data['sizes']:
            size = Size(**jsoned)
            size.token = self.token
            sizes.append(size)
        return sizes

    def get_images(self, private=False, type=None):
        """
            This function returns a list of Image object.
        """
        params = {}
        if private:
            params['private'] = 'true'
        if type:
            params['type'] = type
        data = self.get_data("images/", params=params)
        images = list()
        for jsoned in data['images']:
            image = Image(**jsoned)
            image.token = self.token
            images.append(image)
        return images

    def get_all_images(self):
        """
            This function returns a list of Image objects containing all
            available DigitalOcean images, both public and private.
        """
        images = self.get_images()
        return images

    def get_image(self, image_id_or_slug):
        """
            Return a Image by its ID/Slug.
        """
        return Image.get_object(
            api_token=self.token,
            image_id_or_slug=image_id_or_slug,
        )

    def get_my_images(self):
        """
            This function returns a list of Image objects representing
            private DigitalOcean images (e.g. snapshots and backups).
        """
        images = self.get_images(private=True)
        return images

    def get_global_images(self):
        """
            This function returns a list of Image objects representing
            public DigitalOcean images (e.g. base distribution images
            and 'One-Click' applications).
        """
        data = self.get_images()
        images = list()
        for i in data:
            if i.public:
                i.token = self.token
                images.append(i)
        return images

    def get_distro_images(self):
        """
            This function returns a list of Image objects representing
            public base distribution images.
        """
        images = self.get_images(type='distribution')
        return images

    def get_app_images(self):
        """
            This function returns a list of Image objectobjects representing
            public DigitalOcean 'One-Click' application images.
        """
        images = self.get_images(type='application')
        return images

    def get_all_domains(self):
        """
            This function returns a list of Domain object.
        """
        data = self.get_data("domains/")
        domains = list()
        for jsoned in data['domains']:
            domain = Domain(**jsoned)
            domain.token = self.token
            domains.append(domain)
        return domains

    def get_domain(self, domain_name):
        """
            Return a Domain by its domain_name
        """
        return Domain.get_object(api_token=self.token, domain_name=domain_name)

    def get_all_sshkeys(self):
        """
            This function returns a list of SSHKey object.
        """
        data = self.get_data("account/keys/")
        ssh_keys = list()
        for jsoned in data['ssh_keys']:
            ssh_key = SSHKey(**jsoned)
            ssh_key.token = self.token
            ssh_keys.append(ssh_key)
        return ssh_keys

    def get_ssh_key(self, ssh_key_id):
        """
            Return a SSHKey object by its ID.
        """
        return SSHKey.get_object(api_token=self.token, ssh_key_id=ssh_key_id)

    def get_all_tags(self):
        """
            This method returns a list of all tags.
        """
        data = self.get_data("tags")
        return [
            Tag(token=self.token, **tag) for tag in data['tags']
        ]

    def get_action(self, action_id):
        """
            Return an Action object by a specific ID.
        """
        return Action.get_object(api_token=self.token, action_id=action_id)

    def get_all_floating_ips(self):
        """
            This function returns a list of FloatingIP objects.
        """
        data = self.get_data("floating_ips")
        floating_ips = list()
        for jsoned in data['floating_ips']:
            floating_ip = FloatingIP(**jsoned)
            floating_ip.token = self.token
            floating_ips.append(floating_ip)
        return floating_ips

    def get_floating_ip(self, ip):
        """
            Returns a of FloatingIP object by its IP address.
        """
        return FloatingIP.get_object(api_token=self.token, ip=ip)

    def get_all_load_balancers(self):
        """
            Returns a list of Load Balancer objects.
        """
        data = self.get_data("load_balancers")

        load_balancers = list()
        for jsoned in data['load_balancers']:
            load_balancer = LoadBalancer(**jsoned)
            load_balancer.token = self.token
            load_balancer.health_check = HealthCheck(**jsoned['health_check'])
            load_balancer.sticky_sessions = StickySesions(**jsoned['sticky_sessions'])
            forwarding_rules = list()
            for rule in jsoned['forwarding_rules']:
                forwarding_rules.append(ForwardingRule(**rule))
            load_balancer.forwarding_rules = forwarding_rules
            load_balancers.append(load_balancer)
        return load_balancers

    def get_load_balancer(self, id):
        """
            Returns a Load Balancer object by its ID.

            Args:
                id (str): Load Balancer ID
        """
        return LoadBalancer.get_object(api_token=self.token, id=id)

    def get_certificate(self, id):
        """
            Returns a Certificate object by its ID.

            Args:
                id (str): Certificate ID
        """
        return Certificate.get_object(api_token=self.token, cert_id=id)

    def get_all_certificates(self):
        """
            This function returns a list of Certificate objects.
        """
        data = self.get_data("certificates")
        certificates = list()
        for jsoned in data['certificates']:
            cert = Certificate(**jsoned)
            cert.token = self.token
            certificates.append(cert)

        return certificates

    def get_snapshot(self, snapshot_id):
        """
            Return a Snapshot by its ID.
        """
        return Snapshot.get_object(
            api_token=self.token, snapshot_id=snapshot_id
        )

    def get_all_snapshots(self):
        """
            This method returns a list of all Snapshots.
        """
        data = self.get_data("snapshots/")
        return [
            Snapshot(token=self.token, **snapshot)
            for snapshot in data['snapshots']
        ]

    def get_droplet_snapshots(self):
        """
            This method returns a list of all Snapshots based on Droplets.
        """
        data = self.get_data("snapshots?resource_type=droplet")
        return [
            Snapshot(token=self.token, **snapshot)
            for snapshot in data['snapshots']
        ]

    def get_volume_snapshots(self):
        """
            This method returns a list of all Snapshots based on volumes.
        """
        data = self.get_data("snapshots?resource_type=volume")
        return [
            Snapshot(token=self.token, **snapshot)
            for snapshot in data['snapshots']
        ]

    def get_all_volumes(self):
        """
            This function returns a list of Volume objects.
        """
        data = self.get_data("volumes")
        volumes = list()
        for jsoned in data['volumes']:
            volume = Volume(**jsoned)
            volume.token = self.token
            volumes.append(volume)
        return volumes

    def get_volume(self, volume_id):
        """
            Returns a Volume object by its ID.
        """
        return Volume.get_object(api_token=self.token, volume_id=volume_id)

    def get_all_firewalls(self):
        """
            This function returns a list of Firewall objects.
        """
        data = self.get_data("firewalls")
        firewalls = list()
        for jsoned in data['firewalls']:
            firewall = Firewall(**jsoned)
            firewall.token = self.token
            in_rules = list()
            for rule in jsoned['inbound_rules']:
                in_rules.append(InboundRule(**rule))
            firewall.inbound_rules = in_rules
            out_rules = list()
            for rule in jsoned['outbound_rules']:
                out_rules.append(OutboundRule(**rule))
            firewall.outbound_rules = out_rules
            firewalls.append(firewall)
        return firewalls

    def get_firewall(self, firewall_id):
        """
            Return a Firewall by its ID.
        """
        return Firewall.get_object(
            api_token=self.token,
            firewall_id=firewall_id,
        )

    def __str__(self):
        return "<Manager>"
