REM 生成golang版本的pb文件
.\protoc.exe --go_out=.\..\..\ --proto_path=.\..\ .\..\*.proto
REM 生成golang版本的代码模板和消息号映射文件
.\proto_code_gen.exe -input=.\..\..\pb\*.pb.go -config=.\proto_code_gen.yaml

if exist .\..\..\..\gtestclient (
REM /XD . 表示不复制子目录
REM /XF file1.pb.go file2.pb.go 表示排除指定的文件
	REM 拷贝生成的pb文件到gtestclient
    robocopy .\..\..\pb\ .\..\..\..\gtestclient\pb\ *.pb.go /XD . /XF cmd_server.pb.go manual.go.pb.go server_base.pb.go
	REM 拷贝消息号映射文件到gtestclient
	copy .\..\..\cfgdata\message_command_mapping.json .\..\..\..\gtestclient\cfgdata\message_command_mapping.json
) else (
    echo not find gtestclient
)

if exist .\..\..\..\cshap_client (
    REM 生成proto的C#代码
    .\protoc.exe --csharp_out=.\..\..\..\cshap_client\cshap_client\pb\ --proto_path=.\..\ .\..\*.proto
	REM 拷贝消息号映射文件到gtestclient
	copy .\..\..\cfgdata\message_command_mapping.json .\..\..\..\cshap_client\cshap_client\cfgdata\message_command_mapping.json
	REM 生成C#模板代码
	.\proto_code_gen.exe -input=.\..\..\pb\*.pb.go -config=.\proto_code_gen_csharp.yaml
) else (
    echo not find cshap_client
)

if exist .\..\..\..\unity_client (
    REM 生成proto的C#代码
    .\protoc.exe --csharp_out=.\..\..\..\unity_client\client\project\Assets\Code\pb\ --proto_path=.\..\ .\..\*.proto
	REM 拷贝消息号映射文件到gtestclient
	copy .\..\..\cfgdata\message_command_mapping.json .\..\..\..\unity_client\client\project\Assets\cfgdata\message_command_mapping.json
	REM 生成C#模板代码
	.\proto_code_gen.exe -input=.\..\..\pb\*.pb.go -config=.\proto_code_gen_csharp.yaml
) else (
    echo not find unity_client
)