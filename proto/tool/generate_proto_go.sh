#!/bin/bash

protoc --go_out=.\..\..\ --proto_path=.\..\ .\..\*.proto
.\proto_code_gen -input=.\..\..\pb\*.pb.go -config=.\proto_code_gen.yaml

# Check if the target directory exists
if [ -d "../../../gtestclient" ]; then
    # Create destination directories if they don't exist
    mkdir -p ../../../gtestclient/pb
    mkdir -p ../../../gtestclient/cfgdata
    
    # Copy .pb.go files recursively, excluding specified files
    # Explanation:
    #   find: Search in source directory
    #   -type f: Only look for files
    #   -name "*.pb.go": Match Go protobuf files
    #   ! -name: Exclude specific files
    #   -exec cp: Copy each found file to destination
    find ../../pb -type f -name "*.pb.go" \
        ! -name "cmd_server.pb.go" \
        ! -name "manual.go.pb.go" \
        ! -name "server_base.pb.go" \
        -exec cp {} ../../../gtestclient/pb/ \;
    
    # Copy the JSON configuration file
    cp ../../cfgdata/message_command_mapping.json ../../../gtestclient/cfgdata/
else
    echo "gtestclient directory not found"
fi
