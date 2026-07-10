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

if exist .\..\..\..\gtestclient (
    REM protoc C#
    .\protoc.exe ^
        --csharp_out=.\..\..\..\gtestclient\pb\ ^
        --proto_path=.\..\ ^
        .\..\*.proto
	REM proto_code_gen generate message_command_mapping.json and csharp.cs
	.\proto_code_gen.exe ^
        -p .\..\ ^
        -m .\..\..\..\gtestclient\cfgdata\message_command_mapping.json ^
        -e cfg.proto ^
        -oe server_base.proto ^
        -t csharp.cs.template ^
        -c .\..\..\..\gtestclient\pb\csharp.cs
) else (
    echo not find gtestclient
)

if exist .\..\..\..\cshap_client (
    REM protoc C#
    .\protoc.exe ^
        --csharp_out=.\..\..\..\cshap_client\cshap_client\pb\ ^
        --proto_path=.\..\ ^
        .\..\*.proto
	REM proto_code_gen generate message_command_mapping.json and csharp.cs
	.\proto_code_gen.exe ^
        -p .\..\ ^
        -m .\..\..\..\cshap_client\cshap_client\cfgdata\message_command_mapping.json ^
        -e cfg.proto ^
        -oe server_base.proto ^
        -t csharp.cs.template ^
        -c .\..\..\..\cshap_client\cshap_client\pb\csharp.cs
) else (
    echo not find cshap_client
)

if exist .\..\..\..\unity_client (
    REM protoc C#
    .\protoc.exe ^
        --csharp_out=.\..\..\..\unity_client\client\project\Assets\Code\pb\ ^
        --proto_path=.\..\ ^
        .\..\*.proto
	REM proto_code_gen generate message_command_mapping.json and csharp.cs
	.\proto_code_gen.exe ^
        -p .\..\ ^
        -m .\..\..\..\unity_client\client\project\Assets\cfgdata\message_command_mapping.json ^
        -e cfg.proto ^
        -oe server_base.proto ^
        -t csharp.cs.template ^
        -c .\..\..\..\unity_client\client\project\Assets\Code\pb\csharp.cs
) else (
    echo not find unity_client
)
