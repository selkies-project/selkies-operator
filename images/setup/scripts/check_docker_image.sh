#!/bin/bash
# Limitations of GKE image streaming
# 1. You can't use a Secret to pull container images on GKE versions prior to 1.23.5-gke.1900.
# 2. Container images that use the V2 Image Manifest, schema version 1 are not eligible.
# 3. Container images encrypted with customer-managed encryption keys (CMEK) are not eligible for Image streaming. GKE downloads these images without streaming the data. You can still use CMEK to protect attached persistent disks and custom boot disks in clusters that use Image streaming.
# 4. Container images with empty layers or duplicate layers are not eligible for Image streaming. GKE downloads these images without streaming the data. Check your container image for empty layers or duplicate layers.
# 5. The Artifact Registry repository must be in the same region as your GKE nodes, or in a multi-region that corresponds with the region where your nodes are running. For example:
#       If your nodes are in us-east1, Image streaming is available for repositories in the us-east1 region or the us multi-region since both GKE and Artifact Registry are running in data center locations within the United States.
#       If your nodes are in the northamerica-northeast1 region, the nodes are running in Canada. In this situation, Image streaming is only available for repositories in the same region.
# 6. If your workloads read many files in an image during initialization, you might notice increased initialization times because of the latency added by the remote file reads.
# 7. You might not notice the benefits of Image streaming during the first pull of an eligible image. However, after Image streaming caches the image, future image pulls on any cluster benefit from Image streaming.
# 8. GKE uses the cluster-level configuration to determine whether to enable Image streaming on new node pools created using node auto-provisioning. However, you cannot use workload separation to create node pools with Image streaming enabled when Image streaming is disabled at the cluster level.
# 9. Linux file capabilities such as CAP_NET_RAW are supported with Image streaming in GKE version 1.22.6-gke.300 and later. For previous GKE versions, these capabilities are not available when the image file is streamed, or when the image is saved to the local disk. To avoid potential disruptions, do not use Image streaming for containers with these capabilities in GKE versions prior to 1.22.6-gke.300. If your container relies on Linux file capabilities, it might fail to start with permission denied errors when running with Image streaming enabled.
set -ex
display_usage() {
	
	echo -e "\nUsage: $0 -i \n" 
    echo -e "Argument: \n" 
    echo -e "\t -i: IMAGE_NAME" 
}
if [  $# -le 1 ] 
	then 
		display_usage
		exit 1
fi 

while getopts i:h: flag
do
    case "${flag}" in
        i) IMAGE=${OPTARG};;
        *) display_usage
       exit 1 ;;
    esac
done

# docker pull $IMAGE
DOCKER_SCHEMA_VERSION=$(docker manifest inspect --verbose ${IMAGE} | grep '"schemaVersion": 2,' | wc -l)
LAYERS=$(docker inspect $IMAGE  | jq .[].RootFS.Layers | sort | wc -l)
UNIQUE_LAYERS=$(docker inspect $IMAGE  | jq .[].RootFS.Layers | sort | uniq | wc -l )   
EMPTY_LAYER=$(docker inspect $IMAGE | jq .[].RootFS.Layers | grep -i "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4" | wc -l)

if [[ $DOCKER_SCHEMA_VERSION -eq 0 ]];  then
	echo "[ ERROR ] Image ${IMAGE} failed to match image streaming criteria. Reason: Docker schema version mismatch, reqires schemaVersion: 2"
	echo "[ ERROR ] schemaVersion : $(docker manifest inspect --verbose ${IMAGE} | grep '"schemaVersion"')"
	exit 1
fi

if [[ $LAYERS -ne $UNIQUE_LAYERS ]]; then
	echo "[ ERROR ] Image ${IMAGE} failed to match image streaming criteria. Reason: Duplicate docker layers."
	echo "[ ERROR ] Duplicate layers: $(docker inspect $IMAGE  | jq .[].RootFS.Layers | sort | uniq -d)"
	exit 1
fi

if [[ $EMPTY_LAYER -gt 0 ]]; then
	echo "[ ERROR ] Image ${IMAGE} failed to match image streaming criteria. Reason: Empty docker layers."
	echo "[ ERROR ] Image contains empty layers with sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4" 
        exit 1
fi

echo "[ INFO ] Success!!! Image ${IMAGE} matching criteria for image streaming."