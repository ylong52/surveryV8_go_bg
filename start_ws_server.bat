@echo off
cd /d "e:\case2025\adspower\surveyV8_go\surveryV8_go_bg"
echo Starting ws_server at %date% %time% >> ws_server_output.log
ws_server_bin.exe >> ws_server_output.log 2>&1
