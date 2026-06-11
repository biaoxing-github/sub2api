# 测试脚本：模拟 Codex CLI 请求头发送到 sub2api
# 用途：验证 sub2api 是否正确识别和转发 Codex 请求头

$apiKey = "your-api-key-here"  # 替换为你的测试 API Key
$endpoint = "http://127.0.0.1:3000/v1/chat/completions"  # sub2api 端点

# Codex CLI 的标准请求头（从代码中提取）
$headers = @{
    "Authorization" = "Bearer $apiKey"
    "Content-Type" = "application/json"
    "Accept" = "text/event-stream"
    "User-Agent" = "codex_cli_rs/0.125.0"
    "Originator" = "codex_cli_rs"
    "Version" = "0.125.0"
    "OpenAI-Beta" = "responses=experimental"
}

# 测试请求体
$body = @{
    model = "gpt-4"
    messages = @(
        @{
            role = "user"
            content = "Hello, test from Codex CLI"
        }
    )
    stream = $false
} | ConvertTo-Json -Depth 10

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "发送请求到: $endpoint" -ForegroundColor Cyan
Write-Host "使用 Codex CLI 请求头:" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$headers.GetEnumerator() | ForEach-Object {
    Write-Host "$($_.Key): $($_.Value)" -ForegroundColor Yellow
}

Write-Host "`n发送请求中..." -ForegroundColor Green

try {
    $response = Invoke-WebRequest -Uri $endpoint -Method POST -Headers $headers -Body $body -UseBasicParsing

    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "响应状态: $($response.StatusCode)" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host $response.Content
} catch {
    Write-Host "`n========================================" -ForegroundColor Red
    Write-Host "请求失败!" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red
    Write-Host "错误信息: $($_.Exception.Message)" -ForegroundColor Red

    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "响应内容:" -ForegroundColor Yellow
        Write-Host $responseBody
    }
}
