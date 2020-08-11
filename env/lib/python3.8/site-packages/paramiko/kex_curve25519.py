import binascii
import hashlib

from cryptography.exceptions import UnsupportedAlgorithm
from cryptography.hazmat.primitives import constant_time, serialization
from cryptography.hazmat.primitives.asymmetric.x25519 import (
    X25519PrivateKey,
    X25519PublicKey,
)

from paramiko.message import Message
from paramiko.py3compat import byte_chr, long
from paramiko.ssh_exception import SSHException


_MSG_KEXECDH_INIT, _MSG_KEXECDH_REPLY = range(30, 32)
c_MSG_KEXECDH_INIT, c_MSG_KEXECDH_REPLY = [byte_chr(c) for c in range(30, 32)]


class KexCurve25519(object):
    hash_algo = hashlib.sha256

    def __init__(self, transport):
        self.transport = transport
        self.key = None

    @classmethod
    def is_available(cls):
        try:
            X25519PrivateKey.generate()
        except UnsupportedAlgorithm:
            return False
        else:
            return True

    def _perform_exchange(self, peer_key):
        secret = self.key.exchange(peer_key)
        if constant_time.bytes_eq(secret, b"\x00" * 32):
            raise SSHException(
                "peer's curve25519 public value has wrong order"
            )
        return secret

    def start_kex(self):
        self.key = X25519PrivateKey.generate()
        if self.transport.server_mode:
            self.transport._expect_packet(_MSG_KEXECDH_INIT)
            return

        m = Message()
        m.add_byte(c_MSG_KEXECDH_INIT)
        m.add_string(
            self.key.public_key().public_bytes(
                serialization.Encoding.Raw, serialization.PublicFormat.Raw
            )
        )
        self.transport._send_message(m)
        self.transport._expect_packet(_MSG_KEXECDH_REPLY)

    def parse_next(self, ptype, m):
        if self.transport.server_mode and (ptype == _MSG_KEXECDH_INIT):
            return self._parse_kexecdh_init(m)
        elif not self.transport.server_mode and (ptype == _MSG_KEXECDH_REPLY):
            return self._parse_kexecdh_reply(m)
        raise SSHException(
            "KexCurve25519 asked to handle packet type {:d}".format(ptype)
        )

    def _parse_kexecdh_init(self, m):
        peer_key_bytes = m.get_string()
        peer_key = X25519PublicKey.from_public_bytes(peer_key_bytes)
        K = self._perform_exchange(peer_key)
        K = long(binascii.hexlify(K), 16)
        # compute exchange hash
        hm = Message()
        hm.add(
            self.transport.remote_version,
            self.transport.local_version,
            self.transport.remote_kex_init,
            self.transport.local_kex_init,
        )
        server_key_bytes = self.transport.get_server_key().asbytes()
        exchange_key_bytes = self.key.public_key().public_bytes(
            serialization.Encoding.Raw, serialization.PublicFormat.Raw
        )
        hm.add_string(server_key_bytes)
        hm.add_string(peer_key_bytes)
        hm.add_string(exchange_key_bytes)
        hm.add_mpint(K)
        H = self.hash_algo(hm.asbytes()).digest()
        self.transport._set_K_H(K, H)
        sig = self.transport.get_server_key().sign_ssh_data(H)
        # construct reply
        m = Message()
        m.add_byte(c_MSG_KEXECDH_REPLY)
        m.add_string(server_key_bytes)
        m.add_string(exchange_key_bytes)
        m.add_string(sig)
        self.transport._send_message(m)
        self.transport._activate_outbound()

    def _parse_kexecdh_reply(self, m):
        peer_host_key_bytes = m.get_string()
        peer_key_bytes = m.get_string()
        sig = m.get_binary()

        peer_key = X25519PublicKey.from_public_bytes(peer_key_bytes)

        K = self._perform_exchange(peer_key)
        K = long(binascii.hexlify(K), 16)
        # compute exchange hash and verify signature
        hm = Message()
        hm.add(
            self.transport.local_version,
            self.transport.remote_version,
            self.transport.local_kex_init,
            self.transport.remote_kex_init,
        )
        hm.add_string(peer_host_key_bytes)
        hm.add_string(
            self.key.public_key().public_bytes(
                serialization.Encoding.Raw, serialization.PublicFormat.Raw
            )
        )
        hm.add_string(peer_key_bytes)
        hm.add_mpint(K)
        self.transport._set_K_H(K, self.hash_algo(hm.asbytes()).digest())
        self.transport._verify_key(peer_host_key_bytes, sig)
        self.transport._activate_outbound()
