from common import random_str, auth_check


def test_dns_fields(client):
    auth_check(client.schema, 'dnsRecord', 'crud', {
        'namespaceId': 'cr',
        'projectId': 'cr',
        'hostname': 'cru',
        'ipAddresses': 'cru',
        'clusterIp': 'r',
        'selector': 'cru',
        'targetWorkloadIds': 'cru',
        'workloadId': 'r',
        'targetDnsRecordIds': 'cru',
    })


def test_dns_hostname(pc):
    client = pc.client

    ns = client.create_namespace(name=random_str())

    name = random_str()
    dns_record = client.create_dns_record(name=name,
                                          hostname='target',
                                          namespaceId=ns.id)
    assert dns_record.baseType == 'dnsRecord'
    assert dns_record.type == 'dnsRecord'
    assert dns_record.name == name
    assert dns_record.hostname == 'target'
    assert dns_record.clusterIp is None
    assert dns_record.namespaceId == ns.id
    assert 'namespace' not in dns_record
    assert dns_record.projectId == pc.project.id

    dns_record = client.update(dns_record, hostname='target2')
    dns_record = client.reload(dns_record)

    assert dns_record.baseType == 'dnsRecord'
    assert dns_record.type == 'dnsRecord'
    assert dns_record.name == name
    assert dns_record.hostname == 'target2'
    assert dns_record.clusterIp is None
    assert dns_record.namespaceId == ns.id
    assert 'namespace' not in dns_record
    assert dns_record.projectId == pc.project.id

    found = False
    for i in client.list_dns_record():
        if i.id == dns_record.id:
            found = True
            break

    assert found

    dns_record = client.by_id_dns_record(dns_record.id)
    assert dns_record is not None

    client.delete(dns_record)


def test_dns_ips(pc):
    client = pc.client

    ns = client.create_namespace(name=random_str())

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
    assert dns_record.projectId == pc.project.id

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
    assert dns_record.projectId == pc.project.id

    found = False
    for i in client.list_dns_record():
        if i.id == dns_record.id:
            found = True
            break

    assert found

    dns_record = client.by_id_dns_record(dns_record.id)
    assert dns_record is not None

    client.delete(dns_record)
