REM protoc generate *pb file for golang

.\protoc.exe ^
    --plugin=protoc-gen-go=.\protoc-gen-go.exe ^
    --go_out=.\..\..\ ^
    --proto_path=.\..\ ^
    .\..\*.proto

REM generate message_command_mapping.json for server
.\proto_code_gen.exe ^
    -p .\..\ ^
    -m .\..\..\cfgdata\message_command_mapping.json ^
    -e cfg.proto
