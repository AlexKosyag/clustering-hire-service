# =====================================================================
# Smoke-тест ключевых сценариев API (запуск при поднятом backend на :3000)
#   .\scripts\smoke.ps1 [-BaseUrl http://localhost:3000/api]
# =====================================================================
param([string]$BaseUrl = "http://localhost:3000/api")

$ErrorActionPreference = "Stop"
$results = New-Object System.Collections.ArrayList

function Check([string]$name, [bool]$ok, [string]$detail = "") {
    $status = if ($ok) { "PASS" } else { "FAIL" }
    [void]$results.Add([pscustomobject]@{ Test = $name; Status = $status; Detail = $detail })
    Write-Host ("[{0}] {1} {2}" -f $status, $name, $detail) -ForegroundColor $(if ($ok) { "Green" } else { "Red" })
}

function ErrMsg($e) {
    if ($e.ErrorDetails -and $e.ErrorDetails.Message) { return $e.ErrorDetails.Message }
    return $e.Exception.Message
}

function PostJson([string]$uri, [string]$body, [string]$token = $null) {
    $headers = @{}
    if ($token) { $headers["Authorization"] = "Bearer $token" }
    Invoke-RestMethod -Uri $uri -Method Post -ContentType "application/json" -Body $body -Headers $headers
}

function GetJson([string]$uri, [string]$token = $null) {
    $headers = @{}
    if ($token) { $headers["Authorization"] = "Bearer $token" }
    Invoke-RestMethod -Uri $uri -Method Get -Headers $headers
}

$sw = [System.Diagnostics.Stopwatch]::StartNew()

# ---------- 1. Health ----------
try {
    $health = GetJson "$BaseUrl/health"
    Check "health.status" ($health.status -eq "ok") ("db=" + $health.services.database + " redis=" + $health.services.redis + " embedding=" + $health.services.embedding.provider)
    Check "health.embedding.dim" ($health.services.embedding.dim -eq 384)
} catch { Check "health" $false $_.Exception.Message }

# ---------- 2. Кандидат: вход, профиль ----------
$candidate = PostJson "$BaseUrl/auth/login" '{"email":"test@candidate.com","password":"Test123!"}'
$ct = $candidate.accessToken
Check "auth.candidate.login" ($null -ne $ct) $candidate.user.email

$profile = GetJson "$BaseUrl/candidates/profile" $ct
Check "candidates.profile" ($profile.specialization -eq "BACKEND") ("spec=" + $profile.specialization + " grade=" + $profile.declaredGrade)

# ---------- 3. Testing: полный цикл CAT-теста (на свежезарегистрированном
#    аккаунте — не портит состояние демо-кандидатов и идемпотентен) ----------
try {
    $smokeEmail = "smoke+{0}@example.com" -f ([guid]::NewGuid().ToString("N").Substring(0, 8))
    $tester = PostJson "$BaseUrl/auth/register" ('{"email":"' + $smokeEmail + '","password":"Smoke1234!","role":"CANDIDATE"}')
    $tt = $tester.accessToken
    Check "auth.register" ($null -ne $tt) $smokeEmail

    $sid = $null
    $active = GetJson "$BaseUrl/testing/active" $tt
    if ($active.session) {
        $sid = $active.session.id
        Check "testing.start" $true "найдена активная сессия (повторный запуск)"
    } else {
        try {
            $start = PostJson "$BaseUrl/testing/start" '{"specialization":"DEVOPS","declaredGrade":"JUNIOR"}' $tt
            $sid = $start.sessionId
            Check "testing.start" ($null -ne $sid) ("sessionId=" + $sid)
        } catch {
            if ($_.Exception.Message -match "cooldown|Cooldown|уже|Пересдача|возможна") {
                Check "testing.start" $true "cooldown активен — тест уже проходил (идемпотентный повтор)"
            } else { throw }
        }
    }

    if ($sid) {
    $leaked = $false
    $answers = 0
    for ($i = 0; $i -lt 20; $i++) {
        $current = GetJson "$BaseUrl/testing/session/$sid/current" $tt
        $q = $current.question
        if ($null -eq $q) { break }
        $raw = $current | ConvertTo-Json -Depth 8
        if ($raw -match '"correctAnswer"|"isCorrect"|"explanation"') { $leaked = $true }

        $body = @{ questionId = $q.id; timeSpentSeconds = 5 }
        if ($q.questionType -eq "multiple_choice") {
            $ids = @($q.options | ForEach-Object { $_.id })
            $body["answer"] = $ids[($i % $ids.Count)]
        } else {
            $body["answerText"] = "Ключевые фразы: SELECT JOIN try catch валидация тесты документация"
        }
        $json = $body | ConvertTo-Json -Compress
        PostJson "$BaseUrl/testing/session/$sid/answer" $json $tt | Out-Null
        $answers++
    }
    Check "testing.answer20" ($answers -ge 1) ("answered=" + $answers)
    Check "testing.noAnswerLeak" (-not $leaked)

    $completed = PostJson "$BaseUrl/testing/session/$sid/complete" "{}" $tt
    Check "testing.complete" ($null -ne $completed.score) ("status=" + $completed.status + " score=" + $completed.score + " rec=" + $completed.recommendation + " theta=" + $completed.theta)
    }
} catch {
    Check "testing.flow" $false $_.Exception.Message
}

# ---------- 4. Работодатель: вход, matching-поиск ----------
$employer = PostJson "$BaseUrl/auth/login" '{"email":"test@employer.com","password":"Test123!"}'
$et = $employer.accessToken
Check "auth.employer.login" ($null -ne $et) $employer.user.email

$needBody = @{
    description    = "Ищем backend-разработчика на Node.js с опытом от 3 лет: REST API, PostgreSQL, Redis, очереди, высокая нагрузка"
    specialization = "BACKEND"
    grade          = "MIDDLE"
    stack          = @("Node.js", "PostgreSQL", "Redis", "Docker")
} | ConvertTo-Json -Compress

$sw.Restart()
$search = PostJson "$BaseUrl/matching/search" $needBody $et
$elapsed = $sw.Elapsed.TotalMilliseconds
Check "matching.search" ($search.total -gt 0) ("total=" + $search.total + " time=" + [math]::Round($elapsed, 1) + "ms")
Check "matching.search.latency<500ms" ($elapsed -lt 500) ("{0}ms" -f [math]::Round($elapsed, 1))

if ($search.candidates.Count -gt 0) {
    $top = $search.candidates[0]
    Check "matching.top.score" (($top.matchScore -ge 0) -and ($top.matchScore -le 100)) ("score=" + $top.matchScore + " " + $top.firstName + " " + $top.lastName)
    Check "matching.top.explanation" (($search.candidates[0].matchExplanation | ConvertTo-Json -Depth 6).Length -gt 10)
    $hardFail = @($search.candidates | Where-Object { $_.specialization -ne "BACKEND" }).Count
    Check "matching.hardFilters" ($hardFail -eq 0) ("не-BACKEND в выдаче: " + $hardFail)
}

$sw.Restart()
$semantic = PostJson "$BaseUrl/matching/semantic" '{"query":"разработчик с опытом работы с Redis и очередями сообщений","limit":10}' $et
$semMs = $sw.Elapsed.TotalMilliseconds
Check "matching.semantic" ($semantic.candidates.Count -gt 0) ("hits=" + $semantic.candidates.Count + " time=" + [math]::Round($semMs, 1) + "ms")

$dashboard = GetJson "$BaseUrl/employers/dashboard" $et
Check "employers.dashboard" ($null -ne $dashboard) (($dashboard | ConvertTo-Json -Compress).Substring(0, [Math]::Min(160, ($dashboard | ConvertTo-Json -Compress).Length)))

$candSearch = GetJson "$BaseUrl/employers/candidates/search?specialization=BACKEND&limit=5" $et
Check "employers.candidates.search" (@($candSearch.candidates).Count -gt 0) ("found=" + @($candSearch.candidates).Count)

# ---------- 5. Приглашение: EMPLOYER-first + раскрытие контактов ----------
# Контакты кандидатов скрыты в выдаче (privacy by design) — приглашение делаем
# отдельным работодателем основному demo-кандидату и принимаем его от его имени.
try {
    $emp3 = PostJson "$BaseUrl/auth/login" '{"email":"hr3@example.com","password":"Test123!"}'
    $e3 = $emp3.accessToken
    $inv = $null
    try {
        $inv = PostJson "$BaseUrl/invitations" (@{
                candidateId  = $profile.id
                message      = "Приглашаем обсудить вакансию backend-разработчика"
                salaryFrom   = 250000
                salaryTo     = 350000
            } | ConvertTo-Json -Compress) $e3
        Check "invitations.create" ($null -ne $inv.id) ("id=" + $inv.id)
    } catch {
        if ($_.Exception.Message -match "уже приглашён|409") {
            Check "invitations.create" $true "повторный запуск — приглашение уже существует"
        } else { throw }
    }

    $mine = GetJson "$BaseUrl/invitations" $ct
    Check "invitations.candidate.list" ($mine.invitations.Count -gt 0) ("count=" + $mine.invitations.Count)

    if ($inv) {
        $detail = GetJson "$BaseUrl/invitations/$($inv.id)" $e3
        Check "invitations.contactsLocked" ($detail.candidate.contactsLocked -eq $true) ("contactsLocked=" + $detail.candidate.contactsLocked)

        PostJson "$BaseUrl/invitations/$($inv.id)/accept" "{}" $ct | Out-Null
        $detail2 = GetJson "$BaseUrl/invitations/$($inv.id)" $e3
        Check "invitations.accept.unlocksContacts" ($detail2.candidate.contacts -ne $null) ("email=" + $detail2.candidate.contacts.email)
    }
} catch {
    Check "invitations.flow" $false $_.Exception.Message
}

# ---------- 6. Вакансии и отклики ----------
$vacancies = GetJson "$BaseUrl/vacancies?limit=5&onlyActive=false"
Check "vacancies.list" ($vacancies.total -ge 30) ("total=" + $vacancies.total)

$activeVacancy = @($vacancies.vacancies | Where-Object { $_.isActive }) | Select-Object -First 1
try {
    $apply = PostJson "$BaseUrl/vacancies/$($activeVacancy.id)/apply" '{"coverLetter":"Готов обсудить вакансию"}' $ct
    Check "vacancies.apply" ($null -ne $apply.id) ("applicationId=" + $apply.id)
} catch {
    if ($_.Exception.Message -match "уже отправлен|Conflict|409") {
        Check "vacancies.apply" $true "повторный запуск — отклик уже существует"
    } else { Check "vacancies.apply" $false $_.Exception.Message }
}

$myApps = GetJson "$BaseUrl/applications/mine" $ct
Check "applications.mine" ($myApps.applications.Count -gt 0) ("count=" + $myApps.applications.Count)

# ---------- 7. FSP, уведомления ----------
$fsp = PostJson "$BaseUrl/fsp/connect" '{"fspId":"FSP-1000001"}' $ct
Check "fsp.connect" ($fsp.achievementsFound -gt 0) ("achievements=" + $fsp.achievementsFound)

$notes = GetJson "$BaseUrl/notifications" $ct
Check "notifications" ($notes.total -gt 0) ("total=" + $notes.total)

# ---------- Итог ----------
Write-Host ""
$failed = @($results | Where-Object { $_.Status -eq "FAIL" })
Write-Host ("Итого: {0}, пройдено: {1}, провалено: {2}" -f $results.Count, ($results.Count - $failed.Count), $failed.Count)
if ($failed.Count -gt 0) {
    $failed | Format-Table -AutoSize | Out-String | Write-Host
    exit 1
}
exit 0
