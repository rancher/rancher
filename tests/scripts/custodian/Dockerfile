FROM ubuntu:22.04
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
ARG RANCHER_LINODE_CREDENTIAL
ENV RANCHER_LINODE_CREDENTIAL=$RANCHER_LINODE_CREDENTIAL
ARG OVERRIDE_REGION
ENV OVERRIDE_REGION=$OVERRIDE_REGION
ARG USER_KEYS
ENV USER_KEYS=$USER_KEYS
ARG DONOTDELETE_KEYS
ENV DONOTDELETE_KEYS=$DONOTDELETE_KEYS
ENV CUSTODIAN_YAML=aws.yaml

ADD ./* ./
SHELL ["/bin/bash", "-c"] 

RUN apt-get update && apt-get install -y python3 python3-pip python3-venv apt-transport-https ca-certificates gnupg curl
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] http://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg  add - && apt-get update -y && apt-get install google-cloud-cli -y
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash
RUN python3 -m venv custodian-venv
RUN . custodian-venv/bin/activate
RUN pip3 install c7n c7n_azure c7n_gcp c7n_mailer
RUN az login -u ${AZURE_CLIENT_ID} --service-principal -p ${AZURE_CLIENT_SECRET} --tenant ${AZURE_TENANT_ID}

RUN gcloud auth activate-service-account --key-file google_credentials.json

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