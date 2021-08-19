# Host debuging go-quorum via vscode remote debug & delve

To use vscode to remote debug a running node, the node needs to run in an image
that includes delve, the sources need to be bind mounted into the container,
and the dlv command needs to launch the node process. The compose files have
the necessary plumbing for the bind mounds and the launch

This is an example docker file for preparing a delve image suitable for that
plumbing.  Not sure if the same user patch is still required. Once the image is
built, set it in the .env file as DELVE_IMAGE and switch a node entry in the
docker-compose file to do `<<: *node-debug` instead of `<<: *node-defaults`

