ARG base_image
FROM ${base_image}

USER root

ARG build_id=0
RUN echo ${build_id}

RUN microdnf -y module enable nodejs:{{.NODEJS_VERSION}}
RUN microdnf --setopt=install_weak_deps=0 --setopt=tsflags=nodocs install -y {{.PACKAGES}} && microdnf clean all

RUN echo uid:gid "{{.CNB_USER_ID}}:{{.CNB_GROUP_ID}}"
USER {{.CNB_USER_ID}}:{{.CNB_GROUP_ID}}

RUN echo "CNB_STACK_ID: {{.CNB_STACK_ID}}"