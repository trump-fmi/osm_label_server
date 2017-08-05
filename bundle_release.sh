#!/bin/sh

go build
today=$(date +%G-%m-%d) 
archivename="osm_label_server_${today}.tar.gz"
tar -czf $archivename osm_label_server lib/ start.sh bremen-latest.osm.pbf.ce default.json 
