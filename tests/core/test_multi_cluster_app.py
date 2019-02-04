# from .common import random_str
# import time
#
# def test_multiclusterapp_create(admin_mc, admin_pc):
#     client = admin_mc.client
#     name1 = random_str()
#     templateVersion = "cattle-global-data:library-wordpress-2.1.10"
#
#     mcapp1 = client.create_multi_cluster_app(name=name1,
#                                     templateVersion=templateVersion,
#                                     targets=[admin_pc.project.id],
#                                     roles=["project-member"])
#
#
# def wait_for_app(admin_pc, name, count, timeout=60):
#     start = time.time()
#     interval = 0.5
#     client = admin_pc.client
#     cluster_id, project_id = admin_pc.project.id.split(':')
#     found = False
#     while not found:
#         app = client.by_id_app(name+"-"+project_id)
#         if app is not None:
#             found = True
#             break
#         time.sleep(interval)
#         if time.time() - start > timeout:
#             raise Exception('Timeout waiting for app of multiclusterapp')
#             interval *= 2
