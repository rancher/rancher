from .common import random_str, auth_check


def test_dns_fields(admin_pc_client):
    auth_check(admin_pc_client.schema, 'dnsRecord', 'crud', {
        'namespaceId': 'cr',
        'projectId': 'cr',
        'hostname': 'cru',
        'ipAddresses': 'cru',
        'clusterIp': 'r',
        'selector': 'cru',
        'targetWorkloadIds': 'cru',
        'workloadId': 'r',
        'targetDnsRecordIds': 'cru',
        'publicEndpoints': 'r',
        'ports': 'r',
    })


def test_dns_hostname(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    dns_record = client.create_dns_record(name=name,
                                          hostname='target',
                                          namespaceId=ns.id)
    assert dns_record.baseType == 'dnsRecord'
    assert dns_record.type == 'dnsRecord'
    assert dns_record.name == name
    assert dns_record.hostname == 'target'
    assert "clusterIp" not in dns_record
    assert dns_record.namespaceId == ns.id
    assert 'namespace' not in dns_record
    assert dns_record.projectId == admin_pc.project.id

    dns_record = client.update(dns_record, hostname='target2')
    dns_record = client.reload(dns_record)

    assert dns_record.baseType == 'dnsRecord'
    assert dns_record.type == 'dnsRecord'
    assert dns_record.name == name
    assert dns_record.hostname == 'target2'
    assert "clusterIp" not in dns_record
    assert dns_record.namespaceId == ns.id
    assert 'namespace' not in dns_record
    assert dns_record.projectId == admin_pc.project.id

    found = False
    for i in client.list_dns_record():
        if i.id == dns_record.id:
            found = True
            break

    assert found

    dns_record = client.by_id_dns_record(dns_record.id)
    assert dns_record is not None

    client.delete(dns_record)


def test_dns_ips(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    dns_record = client.create_dns_record(name=name,
                                          ipAddresses=['1.1.1.1',
                                                       '2.2.2.2'],
                                          namespaceId=ns.id)
    assert dns_record.baseType == 'dnsRecord'
    assert dns_record.type == 'dnsRecord'
    assert dns_record.name == name
    assert 'hostname' not in dns_record
    assert dns_record.ipAddresses == ['1.1.1.1', '2.2.2.2']
    assert dns_record.clusterIp is None
    assert dns_record.namespaceId == ns.id
    assert 'namespace' not in dns_record
    assert dns_record.projectId == admin_pc.project.id

    dns_record = client.update(dns_record, ipAddresses=['1.1.1.2', '2.2.2.1'])
    dns_record = client.reload(dns_record)

    assert dns_record.baseType == 'dnsRecord'
    assert dns_record.type == 'dnsRecord'
    assert dns_record.name == name
    assert 'hostname' not in dns_record
    assert dns_record.ipAddresses == ['1.1.1.2', '2.2.2.1']
    assert dns_record.clusterIp is None
    assert dns_record.namespaceId == ns.id
    assert 'namespace' not in dns_record
    assert dns_record.projectId == admin_pc.project.id

    found = False
    for i in client.list_dns_record():
        if i.id == dns_record.id:
            found = True
            break

    assert found

    dns_record = client.by_id_dns_record(dns_record.id)
    assert dns_record is not None

    client.delete(dns_record)
