#!/bin/bash

# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Font colors
export RED='\033[1;31m'
export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export NC='\033[0m' # No Color

export GCLOUD=${GCLOUD:-"gcloud -q"}

function log_red() {
    echo -e "${RED}$@${NC}"
}

function log_cyan() {
    echo -e "${CYAN}$@${NC}"
}

function log_green() {
    echo -e "${GREEN}$@${NC}"
}

function get_user_input() {
    local PROMPT="$1"
    local DEFAULT="$2"
    [[ -z "${PROMPT}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <PROMPT> [<DEFAULT VALUE>]" >&2) && return 1

    while [[ -z "${INPUT}" ]]; do
        if [[ -n "${DEFAULT}" ]]; then
            read -p "$(log_green $PROMPT \(enter for default: ${DEFAULT}\)): " INPUT
            [[ -z "${INPUT}" && -n "${DEFAULT}" ]] && INPUT=${DEFAULT}
        else
            read -p "$(log_green $PROMPT): " INPUT
        fi
    done
    echo "${INPUT}"
}

function cloudbuild_project_owner() {
    local PROJECT_ID=$1
    local PROJECT_NUMBER=$($GCLOUD projects describe ${PROJECT_ID} --format 'value(projectNumber)')
    $GCLOUD services enable cloudbuild.googleapis.com
    $GCLOUD projects add-iam-policy-binding ${PROJECT_ID} \
        --member serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com \
        --role roles/owner >/dev/null
}

function enable_services() {
    ${GCLOUD?env not set} services enable \
        compute.googleapis.com \
        container.googleapis.com \
        cloudbuild.googleapis.com \
        sourcerepo.googleapis.com
}

function get_csr_repo_url() {
    local REPO_NAME=$1
    [[ -z "${REPO_NAME}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <REPO_NAME>" >&2) && return 1

    ${GCLOUD?env not set} source repos describe ${REPO_NAME} --format='value(url)' 2>/dev/null || true
}

function init_csr_repo() {
    local REPO_NAME=$1
    [[ -z "${REPO_NAME}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <REPO_NAME>" >&2) && return 1

    # Skip creation if repo already exists
    [[ -n "$(get_csr_repo_url ${REPO_NAME})" ]] && return 0

    # Create repo
    ${GCLOUD?env not set} source repos create ${REPO_NAME}
}

function delete_csr_repo() {
    local REPO_NAME=$1
    [[ -z "${REPO_NAME}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <REPO_NAME>" >&2) && return 1
    
    # Delete repo
    ${GCLOUD?env not set} source repos delete $REPO_NAME
}

function get_build_trigger_id() {
    local PROJECT_ID=$1
    local REPO_NAME=$2
    local BRANCH=$3
    [[ -z "${PROJECT_ID}" || -z "${REPO_NAME}" || -z "${BRANCH}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <PROJECT_ID> <REPO_NAME> <BRANCH>" >&2) && return 1

    ${GCLOUD?env not set} beta builds triggers list --filter "triggerTemplate.repoName=${REPO_NAME} AND triggerTemplate.projectId=${PROJECT_ID} AND triggerTemplate.branchName=${BRANCH}" --format='value(id)' 2>/dev/null
    return 0
}

function create_build_trigger() {
    local TRIGGER_NAME=$1
    local REPO_NAME=$2
    local BUILD_CONFIG=$3
    local BRANCH=$4
    local SUBSTITUTIONS=$5
    [[ -z "${REPO_NAME}" || -z "${BUILD_CONFIG}" || -z "${BRANCH}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <REPO_NAME> <BUILD_CONFIG> <BRANCH> [<SUBSTITUTIONS KEY=VALUE,...]" >&2) && return 1

    SUBSTITUTIONS_ARG="--substitutions=$SUBSTITUTIONS"

    ${GCLOUD?env not set} beta builds triggers create cloud-source-repositories \
        --repo="${REPO_NAME}" \
        --description="${TRIGGER_NAME}" \
        --branch-pattern="${BRANCH}" \
        --build-config="${BUILD_CONFIG}" \
        ${SUBSTITUTIONS_ARG}
}

function delete_build_trigger() {
    local TRIGGER_NAME=$1
    [[ -z "${TRIGGER_NAME}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <TRIGGER_NAME>" >&2) && return 1

    ${GCLOUD?env not set} beta builds triggers delete "${TRIGGER_NAME}"
}

function init_git_repo() {
    local REPO_DIR=$1
    local USER_EMAIL=$2
    local USER_NAME=$3
    [[ -z "${REPO_DIR}" || -z "${USER_EMAIL}" || -z "${USER_NAME}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <REPO_DIR> <USER_EMAIL> <USER_NAME>" >&2) && return 1

    cd "${REPO_DIR}"

    # Skip init if already initialized.
    if [[ ! -d ".git" ]]; then
        git init
        git config --local user.email "${USER_EMAIL}"
        git config --local user.name "${USER_NAME}"
        git config --local credential.'https://source.developers.google.com'.helper gcloud.sh

        git add . && git commit -am "initial commit"
    fi
}

function init_csr_git_remote() {
    local REPO_DIR=$1
    local REPO_NAME=$2
    local REMOTE_NAME=$3
    [[ -z "${REPO_DIR}" || -z "${REPO_NAME}" || -z "${REMOTE_NAME}" ]] && $(log_red "USAGE: ${FUNCNAME[*]} <REPO_DIR> <REPO_NAME> <REMOTE_NAME>" >&2) && return 1

    cd "${REPO_DIR}"

    REMOTE_URL=$(get_csr_repo_url ${REPO_NAME})

    # Skip if remote has already been added
    CURR_REMOTE_URL=$(git remote get-url ${REMOTE_NAME} 2>/dev/null || true)

    # Add new remote if it does not exist.
    [[ -z "${CURR_REMOTE_URL}" ]] && git remote add ${REMOTE_NAME} "${REMOTE_URL}"

    # Update URL if it does not match
    [[ "${CURR_REMOTE_URL}" != "${REMOTE_URL}" ]] && git remote set-url ${REMOTE_NAME} "${REMOTE_URL}"

    git push ${REMOTE_NAME} master
}