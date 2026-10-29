param(
    [string]$BaseUrl = "http://localhost:8090",
    [string]$Username = "",
    [string]$Password = ""
)

$ErrorActionPreference = "Stop"
$envFile = Join-Path $PSScriptRoot "..\deploy\.env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*LOCAL_CLOUD_ADMIN_USERNAME=(.+)$') { if (-not $Username) { $Username = $Matches[1].Trim() } }
        if ($_ -match '^\s*LOCAL_CLOUD_ADMIN_PASSWORD=(.+)$') { if (-not $Password) { $Password = $Matches[1].Trim() } }
    }
}
if (-not $Username) { $Username = "admin" }
if (-not $Password) { $Password = "admin" }

function Invoke-LocalCloudJson {
    param([string]$Method, [string]$Path, [hashtable]$Body = $null, [string]$Token = $null)
    $headers = @{ Accept = "application/json" }
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $uri = "$BaseUrl$Path"
    if ($Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers -ContentType "application/json" -Body ($Body | ConvertTo-Json -Compress)
    }
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
}

Write-Host "Login as $Username..."
$login = Invoke-LocalCloudJson -Method POST -Path "/api/v1/auth/login" -Body @{ username = $Username; password = $Password }
$token = $login.access_token
if (-not $token) { throw "Login failed: no access_token" }

Write-Host "GET /auth/me..."
$me = Invoke-LocalCloudJson -Method GET -Path "/api/v1/auth/me" -Token $token
$rootFolder = $me.root_folder_id
$used = [int64]$me.storage_used_bytes
$origQuota = [int64]$me.storage_quota_bytes

Write-Host "Quota check (temporarily tight quota)..."
$tightQuota = $used + 512
Invoke-LocalCloudJson -Method PATCH -Path "/api/v1/admin/users/$($me.id)" -Token $token -Body @{ storage_quota_bytes = $tightQuota } | Out-Null
try {
    Invoke-LocalCloudJson -Method POST -Path "/api/v1/uploads" -Token $token -Body @{
        folder_id = $rootFolder
        name = "quota-probe.bin"
        expected_size_bytes = 4096
        conflict_policy = "cancel"
    } | Out-Null
    throw "Expected upload init to fail with quota exceeded"
} catch {
    if ($_.Exception.Response.StatusCode.value__ -ne 507) {
        throw "Expected HTTP 507 QUOTA_EXCEEDED, got: $_"
    }
    Write-Host "  OK: upload rejected (507)"
} finally {
    Invoke-LocalCloudJson -Method PATCH -Path "/api/v1/admin/users/$($me.id)" -Token $token -Body @{ storage_quota_bytes = $origQuota } | Out-Null
}

Write-Host "POST /admin/rebuild..."
Invoke-LocalCloudJson -Method POST -Path "/api/v1/admin/rebuild" -Token $token | Out-Null
Start-Sleep -Seconds 2
$report = Invoke-LocalCloudJson -Method GET -Path "/api/v1/admin/rebuild" -Token $token
if (-not $report.finished_at) {
    throw "Rebuild report has no finished_at"
}
Write-Host "  OK: rebuild finished (inserted=$($report.files_inserted) repaired=$($report.files_repaired) missing=$($report.files_missing))"

Write-Host "GET /file-categories and code list..."
$cats = Invoke-LocalCloudJson -Method GET -Path "/api/v1/file-categories" -Token $token
if ($null -eq $cats) { throw "file-categories returned null" }
Write-Host "  OK: $($cats.Count) categories"
try {
    $null = Invoke-LocalCloudJson -Method GET -Path "/api/v1/file-categories/code/files" -Token $token
    Write-Host "  OK: code category list"
} catch {
    throw "file-categories/code/files failed: $_"
}

Write-Host "GET /client/features..."
$feat = Invoke-LocalCloudJson -Method GET -Path "/api/v1/client/features" -Token $token
Write-Host "  OK: tls_enabled=$($feat.tls_enabled) onlyoffice=$([bool]$feat.onlyoffice_public_url)"

Write-Host "All checks passed."
