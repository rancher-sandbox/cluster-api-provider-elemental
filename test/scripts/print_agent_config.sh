#!/bin/bash

while getopts ':n:r:h' opt; do
  case "$opt" in
    n)
      namespace=${OPTARG}
      ;;

    r)
      registration=${OPTARG}
      ;;
   
    ?|h)
      echo "Usage: $(basename $0) [-n my-namespace] [-r my-registration]"
      exit 1
      ;;
  esac
done
shift "$(($OPTIND -1))"

kubectl -n ${namespace} get elementalregistration ${registration} -o yaml | yq '{"agent":.spec.config.elemental.agent, "registration":.spec.config.elemental.registration}'
