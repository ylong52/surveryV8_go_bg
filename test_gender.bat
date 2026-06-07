@echo off
set WS_PORT=10502
start /B cmd /c "go run ws_server\main.go"
timeout /t 6 /nobreak
go run tests\post_ws.go --url="https://escalentcspacehandp.qualtrics.com/jfe/form/SV_eM9M11Pt2XiyUNo" --html="docs\sescalentcspacehandp.qualtrics.comjfeformSV_eM9M11Pt2XiyUNo\Please indicate your gender.txt"
