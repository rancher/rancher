from .common import random_str
from rancher import ApiError


def test_role_template_creation(admin_mc, remove_resource):
    rt_name = random_str()
    rt = admin_mc.client.create_role_template(name=rt_name)
    remove_resource(rt)
    assert rt is not None
    assert rt.name == rt_name


def test_administrative_role_template_creation(admin_mc, remove_resource):
    client = admin_mc.client
    crt_name = random_str()
    crt = client.create_role_template(name=crt_name,
                                      context="cluster",
                                      administrative=True)
    remove_resource(crt)
    assert crt is not None
    assert crt.name == crt_name

    prt_name = random_str()
    try:
        client.create_role_template(name=prt_name,
                                    context="project",
                                    administrative=True)
    except ApiError as e:
        assert e.error.status == 500
        assert e.error.message == "Only cluster roles can be administrative"


def test_edit_builtin_role_template(admin_mc, remove_resource):
    client = admin_mc.client
    # edit non builtin role, any field is updatable
    org_rt_name = random_str()
    rt = client.create_role_template(name=org_rt_name)
    remove_resource(rt)
    assert rt is not None
    assert rt.name == org_rt_name
    new_rt_name = random_str()
    rt = client.update(rt, name=new_rt_name)
    assert rt.name == new_rt_name

    # edit builtin role, only locked,cluster/projectcreatordefault
    # are updatable
    new_rt_name = "Cluster Member-Updated"
    cm_rt = client.by_id_role_template("cluster-member")
    rt = client.update(cm_rt, name=new_rt_name)
    assert rt.name == "Cluster Member"
