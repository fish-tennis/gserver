protoc.exe --go_out=.\..\..\ --proto_path=.\..\ .\..\*.proto
proto_code_gen -input=.\..\..\pb\*.pb.go -config=.\code_templates.json

if exist .\..\..\..\gtestclient (
REM /XD . 表示不复制子目录
REM /XF file1.pb.go file2.pb.go 表示排除指定的文件
    robocopy .\..\..\pb\ .\..\..\..\gtestclient\pb\ *.pb.go /XD . /XF cmd_server.pb.go manual.go.pb.go server_base.pb.go
) else (
    echo not find gtestclient
)