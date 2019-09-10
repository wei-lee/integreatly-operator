#!/usr/bin/env bash
echo $1
export AWS_ACCESS_KEY_ID=$(oc get secret integreatly-cloud-credentials -n $1 -o go-template='{{index .data  "aws_access_key_id"}}' | base64 -D)
export AWS_SECRET_ACCESS_KEY=$(oc get secret integreatly-cloud-credentials -n $1 -o go-template='{{index .data  "aws_secret_access_key"}}' | base64 -D)