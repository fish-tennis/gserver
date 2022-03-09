protoc.exe --go_out=.\..\ --proto_path=.\ .\*.proto
proto_code_gen -input=.\..\pb\*.pb.go -config=.\code_templates.json