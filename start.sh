#!/bin/bash
if [[ "$1" == "" ]]
then
LD_LIBRARY_PATH=./lib  ./osm_label_server
else
LD_LIBRARY_PATH=./lib ./osm_label_server -ce $1
fi
