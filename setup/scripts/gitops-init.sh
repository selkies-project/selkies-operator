#!/bin/bash

# Google LLC 2019
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

set -e

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")
SCRIPT_FILE_NAME=$(basename $0)

# Derive app name from script file name if not set in environment variable.
# This is used for naming convention of the source repos
APP_NAME=${APP_NAME:-${SCRIPT_FILE_NAME//-gitops-init.sh}}

# Source utility functions
source ${SCRIPT_DIR}/util.bash

###
# Get user inputs
###
DEFAULT_PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
[[ -z ${PROJECT_ID} ]] && export PROJECT_ID=$(get_user_input "Enter project ID" ${DEFAULT_PROJECT_ID})
log_cyan "Using project ID: $(log_green ${PROJECT_ID})"

DEFAULT_REPO_NAME="${APP_NAME}-app"
[[ "${APP_NAME}" == "pod-broker" ]] && DEFAULT_REPO_NAME="${APP_NAME}-infrastructure"
[[ -z ${REPO_NAME} ]] && export REPO_NAME=$(get_user_input "Enter desired Cloud Source Repository name for the source repo" ${DEFAULT_REPO_NAME})
log_cyan "Using source repository: $(log_green ${REPO_NAME})"

REPO_BRANCH="master"
    # Infrastructure repo is relative to script location 
    REPO_DIR="${SCRIPT_DIR}/../"

    if [[ "${APP_NAME}" == "pod-broker" ]]; then
    # Collect values for infrastucture repo
    [[ -z ${COOKIE_SECRET} ]] && export COOKIE_SECRET=$(get_user_input "Enter broker cookie secret")
    [[ -z ${CLIENT_ID} ]] && export CLIENT_ID=$(get_user_input "Enter oauth client id")
    [[ -z ${CLIENT_SECRET} ]] && export CLIENT_ID=$(get_user_input "Enter oauth client secret")
else
    # App repo is current directory
    REPO_DIR="${PWD}"
fi


if [[ "${APP_NAME}" == "pod-broker" ]]; then
    # Use substitutions from user inputs for infrastructure.
    SUBSTITUTIONS="_OAUTH_CLIENT_ID=$CLIENT_ID,_OAUTH_CLIENT_SECRET=$CLIENT_SECRET,_BROKER_COOKIE_SECRET=$COOKIE_SECRET"
else
    # Use substitutions from cli args for apps
    SUBSTITUTIONS=$@
fi

# Verify cloudbuild.yaml is present.
[[ ! -e "${REPO_DIR}/cloudbuild.yaml" ]] && log_red "Missing cloudbuild.yaml in ${REPO_DIR}" && exit 1

# Name of git remote users push to that will trigger cloud build.
GITOPS_REMOTE_NAME="gitops"

# Get active gcloud account used for configuring local git user name and email. 
GCLOUD_ACCOUNT=$(gcloud config get-value account 2>/dev/null)
GCLOUD_USER=${GCLOUD_ACCOUNT%@*}

###
# Configure gcloud command
###
export GCLOUD="gcloud --project ${PROJECT_ID} -q"

###
# Check for cleanup option.
###
CLEANUP=0
[[ ${@,,} =~ --cleanup ]] && CLEANUP=1

if [[ $CLEANUP -eq 1 ]]; then
    CONFIRMATION=$(get_user_input "Remove all build triggers and source repository? (yes/no)" "no")

    # Exit if no confirmation.
    [[ ! "${CONFIRMATION,,}" == "yes" ]] && log_red "Aborting cleanup" && exit 1

    log_cyan "Deleting Cloud Build source repo trigger"
    TRIGGER_ID=$(get_build_trigger_id $PROJECT_ID $REPO_NAME "^$REPO_BRANCH\$")
    log_red "$TRIGGER_ID"
    [[ -n "${TRIGGER_ID}" ]] && for id in $TRIGGER_ID; do delete_build_trigger $id; done

    log_cyan "Deleting Cloud Source Repository ${REPO_NAME}"
    CSR_REPO_URL=$(get_csr_repo_url ${REPO_NAME})
    [[ -n ${CSR_REPO_URL} ]] && delete_csr_repo ${REPO_NAME}
    
    log_cyan "Removing .git directory from ${REPO_DIR}"
    rm -rf "${REPO_DIR}/.git"

    exit
fi

CONFIRMATION=$(get_user_input "continue to create repository? (yes/no)" "no")
# Exit if no confirmation.
[[ ! "${CONFIRMATION,,}" == "yes" ]] && log_red "Aborting" && exit 1

###
# Configure project services and Cloud Build permissions
###
log_cyan "Enabling services"
enable_services

log_cyan "Making Cloud Build service account project owner"
cloudbuild_project_owner ${PROJECT_ID}

###
# Set up the source repo
###
log_cyan "Creating Cloud Source Repository: $(log_green ${REPO_NAME})"
init_csr_repo ${REPO_NAME}

log_cyan "Creating Cloud Build trigger for ${REPO_NAME} repository"
TRIGGER_ID=$(get_build_trigger_id ${PROJECT_ID} ${REPO_NAME} "^${REPO_BRANCH}\$")
[[ -z ${TRIGGER_ID} ]] && create_build_trigger "push-to-master" ${REPO_NAME} "cloudbuild.yaml" "^${REPO_BRANCH}\$" $SUBSTITUTIONS

log_cyan "Initializing git repo in ${REPO_DIR} directory"
init_git_repo ${REPO_DIR} ${GCLOUD_ACCOUNT} ${GCLOUD_USER}

log_cyan "Initializing CSR git remote '${GITOPS_REMOTE_NAME}' and pushing first commit"
init_csr_git_remote ${REPO_DIR} ${REPO_NAME} ${GITOPS_REMOTE_NAME}

log_green "View active builds here: https://console.cloud.google.com/cloud-build/builds?project=${PROJECT_ID}"