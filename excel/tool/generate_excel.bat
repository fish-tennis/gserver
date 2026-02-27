REM export excel data
.\excelexporter.exe -config=exporter_server.yaml

if exist .\..\..\..\cshap_client (
	.\excelexporter.exe -config=exporter_csharp.yaml
) else (
    echo not find cshap_client
)

if exist .\..\..\..\unity_client (
	.\excelexporter.exe -config=exporter_unity.yaml
) else (
    echo not find unity_client
)