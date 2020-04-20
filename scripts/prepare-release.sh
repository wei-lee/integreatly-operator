#!/usr/bin/env bash
set -e
set -o pipefail

PREVIOUS_TAG="$(cat deploy/olm-catalog/integreatly-operator/integreatly-operator.package.yaml | grep integreatly-operator | awk -F v '{print $2}')"

check_prerequisites() {
  if [[ -z "${TAG}" ]]; then
    printf "ERROR: no TAG set"
    exit 1
  fi

  if [[ "${TAG}" == "${PREVIOUS_TAG}" ]]; then
    printf "ERROR: csv version ${TAG} already exists"
    exit 1
  fi
}

create_new_csv() {
  cp -r deploy/olm-catalog/integreatly-operator/integreatly-operator-${PREVIOUS_TAG} deploy/olm-catalog/integreatly-operator/${PREVIOUS_TAG}
  operator-sdk generate csv --csv-version "$TAG" --default-channel --csv-channel=rhmi --update-crds --from-version "$PREVIOUS_TAG"
}

move_generated_csv_to_folder() {
  mv deploy/olm-catalog/integreatly-operator/${TAG} deploy/olm-catalog/integreatly-operator/integreatly-operator-${TAG}
}

set_new_version() {
  sed -i "s/image:.*/image: quay\.io\/integreatly\/integreatly-operator:v$TAG/g" deploy/operator.yaml
  sed -i "s/$PREVIOUS_TAG/$TAG/g" version/version.go
  sed -i "s/$PREVIOUS_TAG/$TAG/g" Makefile
}

clean_up() {
  rm -rf deploy/olm-catalog/integreatly-operator/${PREVIOUS_TAG}
}

check_prerequisites
create_new_csv
move_generated_csv_to_folder
set_new_version
clean_up
