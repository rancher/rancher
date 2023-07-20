FROM registry.suse.com/bci/python:3.10
WORKDIR /usr/app/src

ARG AWS_ACCESS_KEY_ID
ENV AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID
ARG AWS_SECRET_ACCESS_KEY
ENV AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY
ARG AZURE_SUBSCRIPTION_ID
ENV AZURE_SUBSCRIPTION_ID=$AZURE_SUBSCRIPTION_ID
ARG AZURE_TENANT_ID
ENV AZURE_TENANT_ID=$AZURE_TENANT_ID
ARG AZURE_CLIENT_ID
ENV AZURE_CLIENT_ID=$AZURE_CLIENT_ID
ARG AZURE_CLIENT_SECRET
ENV AZURE_CLIENT_SECRET=$AZURE_CLIENT_SECRET
ARG RANCHER_GKE_CREDENTIAL
ENV RANCHER_GKE_CREDENTIAL=$RANCHER_GKE_CREDENTIAL
ARG RANCHER_LINODE_CREDENTIAL
ENV RANCHER_LINODE_CREDENTIAL=$RANCHER_LINODE_CREDENTIAL
ARG OVERRIDE_REGION
ENV OVERRIDE_REGION=$OVERRIDE_REGION
ARG USER_KEYS
ENV USER_KEYS=$USER_KEYS
ARG DONOTDELETE_KEYS
ENV DONOTDELETE_KEYS=$DONOTDELETE_KEYS
ENV CUSTODIAN_YAML=aws.yaml


# RUN zypper install -y python3-venv python3-pip vim
RUN python3 -m venv custodian-venv
RUN . custodian-venv/bin/activate
RUN pip3 install c7n c7n_azure c7n_gcp c7n_mailer

ADD ./* ./

# get user keys, replace holder params in yaml files appropriately
SHELL ["/bin/bash", "-c"] 
# think about how to override the keys for the use case of adding tags

CMD for i in `ls *.yaml`; \
    do cat $i | sed -e 's/USERKEYS/'"${USER_KEYS}"'/g' -i $i; done ; \
    for i in `ls *.yaml`; \
    do cat $i | sed -e 's/DONOTDELETEKEYS/'"${DONOTDELETE_KEYS}"'/g' -i $i; done; \
    if [ "$CUSTODIAN_YAML" = "aws.yaml" ] ; then \
    custodian run --output-dir=. --region="us-east-2" --dry-run ${CUSTODIAN_YAML}; \
    custodian run --output-dir=. --region="us-east-1" --dry-run ${CUSTODIAN_YAML}; \
    custodian run --output-dir=. --region="us-west-1" --dry-run ${CUSTODIAN_YAML}; \
    custodian run --output-dir=. --region="us-west-2" --dry-run ${CUSTODIAN_YAML}; \
    elif [ "$CUSTODIAN_YAML" = "tag-to-save.yaml" ] ; then \
    custodian run --output-dir=. --region=$OVERRIDE_REGION --dry-run ${CUSTODIAN_YAML};  \
    else custodian run --output-dir=. --dry-run ${CUSTODIAN_YAML}; fi; 