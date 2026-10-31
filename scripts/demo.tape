Output docs/demo.gif
Set FontSize 14
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"
Type "curl -s http://localhost:8080/healthz"
Enter
Sleep 1s
Type "# runaway agent loop hitting Fuse hard limit"
Enter
Type "for i in $(seq 1 20); do curl -s -o /tmp/out -w '%{http_code}\n' -X POST http://localhost:8080/v1/chat/completions -H 'Content-Type: application/json' -H 'X-Fuse-Agent: runaway' -d '{\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}'; done"
Enter
Sleep 3s
Type "tail -c 200 /tmp/out; echo"
Enter
Sleep 2s
