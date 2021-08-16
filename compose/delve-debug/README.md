# Host debuging go-quorum via vscode remote debug & delve

Example docker file for preparing a delve image.  To make use of host debuging
an image with delve needs to be prepared. Not sure if the same user patch is
still required. Once the image is built, set it in the .env file as DELVE_IMAGE
and switch a node entry in the docker-compose file to do `<<: *node-debug`
instead of `<<: *node-defaults`

