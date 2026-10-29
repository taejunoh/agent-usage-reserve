# Agent Usage Reserve

`agent-usage-reserve`는 Codex와 Claude Code의 전역 지침에 사용량 여유분 정책을 추가하고 관리하는 비공식 로컬 도구입니다. 서비스, 텔레메트리, API 키 접근, 자격 증명 수집, 백그라운드 폴링이 없습니다.

## 데모

![격리된 데모 프로필에서 실행한 실제 CLI 출력](docs/assets/demo.gif)

실제 CLI 출력 · 두 에이전트가 모두 있는 격리된 데모 프로필입니다. 이 데모는 에이전트 지침을 설정하며 사용량을 강제 제한하지 않습니다. 임시 경로는 읽기 쉽도록 줄였습니다.

## 빠른 시작

macOS 또는 Linux에서는 Homebrew로 설치하고 정책을 활성화하세요.

```sh
brew install taejunoh/tap/agent-usage-reserve
agent-usage-reserve enable
```

Homebrew는 실행 파일만 설치하며, `enable`이 정책을 활성화합니다. 한 번에 설치와 활성화를 하려면 macOS/Linux에서 직접 설치기를 사용할 수 있습니다.

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.sh | sh
```

Windows PowerShell에서는 다음 명령을 사용하세요.

```powershell
irm https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.ps1 | iex
```

`enable`은 Codex 홈(`~/.codex`, 또는 `CODEX_HOME`)과 Claude 홈(`~/.claude`, 또는 `CLAUDE_CONFIG_DIR`)을 찾습니다. 하나만 찾으면 그곳에 정책을 적용하고, 둘 다 찾았는데 아직 어느 쪽도 활성화하지 않았다면 둘 다, Claude만, Codex만 중에서 고르라고 묻습니다(Enter를 누르면 둘 다). `enable`을 반복해도 이미 활성화된 에이전트는 그대로 유지되며, 새로 발견된 에이전트를 추가하기 전에는 먼저 묻습니다. 질문 없이 진행하려면 `--only claude`, `--only codex`, `--only all`을 사용하세요. 대화형 터미널이 없으면 아무것도 변경하지 않고 실행할 `--only` 명령을 출력하며, 직접 설치기는 이 경우에도 실행 파일은 설치됐다고 알려줍니다.

직접 설치기는 릴리스 체크섬과 실행 파일 버전을 확인하고 사용자 디렉터리에 설치한 뒤 `enable`을 자동 실행합니다. `sudo`를 사용하거나 셸 시작 파일을 수정하지 않습니다. Unix 경로는 `~/.local/bin`, Windows 경로는 `%LOCALAPPDATA%/Programs/agent-usage-reserve/bin`입니다. Windows에서는 출력된 디렉터리를 사용자 `PATH`에 추가하거나 현재 PowerShell 세션에서 출력된 `$env:Path = "...;$env:Path"` 명령을 실행하세요.

릴리스 실행 파일을 설치하고 실행하는 데 Go, Node, Python, API 키는 필요하지 않습니다.

## 업데이트

직접 설치를 업데이트하려면 해당 플랫폼 설치기를 다시 실행하세요. 설치기는 기준값 없이 `enable`을 실행하므로 활성화된 에이전트별 사용자 지정 기준을 유지합니다.

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.sh | sh
```

Windows PowerShell 설치도 다시 실행하세요.

```powershell
irm https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.ps1 | iex
```

Homebrew는 탭과 formula를 업데이트한 뒤 정책 상태를 확인하고, 비활성화되어 있으면 다시 활성화하세요.

```sh
brew update
brew upgrade taejunoh/tap/agent-usage-reserve
agent-usage-reserve status
agent-usage-reserve enable  # status가 비활성 상태일 때 실행; 현재 기준 유지
```

## 명령

```text
agent-usage-reserve enable  [--threshold 10] [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve set 15  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve status  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve disable [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve --version
```

기준은 1–99 사이의 정수 퍼센트이며 에이전트마다 따로 저장됩니다. `enable`을 기준값 없이 반복하면 각 에이전트의 현재 기준을 유지합니다. `--only` 없이 `set`과 `disable`을 실행하면 활성화된 모든 에이전트에 적용되며, `set`은 활성화된 정책을 변경합니다. `status`는 감지된 모든 에이전트의 관리 정책과 대상 파일을 보여주며 계정 사용량이나 모델 동작을 확인하지 않습니다.

도구는 표시된 정책 블록만 변경합니다. Codex는 Codex 홈의 전역 `AGENTS.override.md` 또는 `AGENTS.md` 중 유효한 파일이고, Claude는 Claude 홈의 `CLAUDE.md`입니다. 변경 전에 원본을 해당 에이전트 홈 디렉터리의 비공개 백업 파일로 저장합니다. `disable`은 다른 텍스트를 보존하면서 관리 블록을 제거합니다. 파일을 바꾸기 전에 작업 중 대상 파일이 변경됐는지 확인해 많은 외부 편집을 감지하지만, 별도 편집기를 직렬화하거나 모든 동시 쓰기를 반드시 감지하지는 못합니다. 정책 비활성화와 실행 파일 삭제는 별도 작업입니다. `disable` 후 바이너리를 삭제하거나 `brew uninstall taejunoh/tap/agent-usage-reserve`를 실행하세요.

### 백업 복구

관리 블록 제거는 먼저 `agent-usage-reserve disable`을 사용하세요. 백업 파일 이름은 `.agent-usage-reserve-backup-*`이며 각 에이전트의 홈 디렉터리(Codex 홈, Claude 홈)에 있습니다. macOS/Linux에서 확인하려면:

```sh
codex_home=${CODEX_HOME:-"$HOME/.codex"}
claude_home=${CLAUDE_CONFIG_DIR:-"$HOME/.claude"}
find "$codex_home" "$claude_home" -type f -name '.agent-usage-reserve-backup-*' -print
```

선택한 백업과 현재 대상 파일을 먼저 살펴보세요. 전체 파일을 복원하면 백업 이후의 편집 내용이 사라질 수 있습니다. 의도적으로 전체 복원하기 전 현재 대상 파일도 복사해 두세요. 백업은 그것이 들어 있는 홈의 에이전트 파일에만 복원해야 합니다. Codex는 `status`가 출력한 실제 활성 파일(`AGENTS.md` 또는 `AGENTS.override.md`)을, Claude는 `CLAUDE.md`를 대상으로 선택합니다.

```sh
backup='/full/path/to/.agent-usage-reserve-backup-...'
target="$codex_home/AGENTS.md" # AGENTS.override.md가 활성 Codex 파일이면 그 경로 사용
# Claude 백업이라면: target="$claude_home/CLAUDE.md"
diff -u "$target" "$backup" || true
cp -p "$target" "$target.before-restore"
cp -p "$backup" "$target" # 전체 파일 복원을 의도한 경우에만 실행
```

Windows PowerShell에서 백업을 확인하려면:

```powershell
$codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME '.codex' }
$claudeHome = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME '.claude' }
Get-ChildItem -Force -File $codexHome, $claudeHome -Filter '.agent-usage-reserve-backup-*' |
  Sort-Object LastWriteTime -Descending
```

```powershell
$backup = 'C:\full\path\to\.agent-usage-reserve-backup-...'
$target = Join-Path $codexHome 'AGENTS.md' # AGENTS.override.md가 활성 Codex 파일이면 그 경로 사용
# Claude 백업이라면: $target = Join-Path $claudeHome 'CLAUDE.md'
Get-FileHash -LiteralPath $backup, $target
Copy-Item -LiteralPath $target -Destination "$target.before-restore"
Copy-Item -LiteralPath $backup -Destination $target -Force # 전체 파일 복원을 의도한 경우에만 실행
```

나중에 추가한 내용을 보존하려면 필요한 부분만 골라 직접 반영하세요. 이름 변경 이전의 백업은 `.codex-usage-reserve-backup-*`이며 도구는 이 파일을 그대로 둡니다. 같은 절차를 해당 이름으로 적용하면 됩니다.

## 적용 범위와 제한

정책은 일반 작업 시작 전과 주요 단계 사이에 사용할 수 있는 계정 사용량 제한을 확인하도록 에이전트에 안내합니다. 실제 예약 작업 실행은 예외입니다. 명시적인 계속 요청은 지정한 작업에만 적용하며, 작업을 지정하지 않으면 현재 작업에만 적용됩니다. 사용자가 임시 기준을 지정하면 그 값을 우선합니다. 서브에이전트는 이 확인을 건너뛰며, 메인 세션이 처리합니다.

이는 행동 지침이며 계정 사용량을 강제로 제한하지 않습니다. 현재 환경에서 제공되는 사용량 데이터와 도구에만 의존하므로 모든 실행에서 사용할 수 있다고 보장하지 않습니다. 데이터가 없으면 추측하지 않고 알립니다. 사용자의 모델 선택·위임 정책은 포함하지 않습니다.

- **Codex:** Codex 데스크톱이나 작업 환경이 계정 사용량 제한을 제공하는 곳에서 동작하며, 모든 CLI 또는 예약 실행에서 사용할 수 있다고 보장하지 않습니다.
- **Claude Code:** 사용량 도구를 제공하는 Claude 데스크톱 앱의 Code 탭에서 동작합니다. 터미널의 `claude` CLI에는 사용량 도구가 없으므로, 그곳에서는 정책이 사용량을 확인할 수 없다고 알리는 데 그칩니다.

## 기존 codex-usage-reserve 사용자

이 도구는 `codex-usage-reserve`에서 이름이 바뀌었으며 이제 Codex와 Claude Code를 함께 관리합니다.

- **Homebrew:** `brew update` 후 `brew upgrade taejunoh/tap/agent-usage-reserve`를 실행하세요. tap이 이전 formula 이름을 새 이름으로 연결하므로 업그레이드하면 자동으로 이전됩니다.
- **직접 설치기:** 설치기를 다시 실행하세요. 설치 디렉터리의 이전 `codex-usage-reserve` 실행 파일을 삭제합니다(Windows에서는 이전 기본 디렉터리에서도 삭제하며, 그 디렉터리를 사용자 `PATH`에서 제거하라고 안내합니다). 이전 실행 파일은 새 정책 표시를 인식하지 못하기 때문입니다.
- **기존 정책**은 계속 동작합니다. 다음에 `enable` 또는 `set`을 실행할 때 기준을 유지한 채 새 표시로 변환됩니다.
- **업그레이드한 뒤 `agent-usage-reserve enable`을 한 번 실행하세요**(Homebrew 사용 여부와 관계없이). `status`에는 이미 Codex가 활성화된 것으로 보이지만, `enable`을 실행해야 Claude가 추가되고 이전 표시가 새 표시로 바뀝니다.
- 이전 `.codex-usage-reserve-backup-*` 파일은 그대로 보존됩니다.

저장소 이름이 바뀐 뒤에도 GitHub 리디렉션 덕분에 기존 한 줄 설치 명령은 계속 동작합니다.

## 개발자용

### 저장소에서 빌드하고 활성화

소스 빌드에는 Go 1.23 이상이 필요합니다.

```sh
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/agent-usage-reserve" ./cmd/agent-usage-reserve
"$HOME/.local/bin/agent-usage-reserve" enable
```

기본 기준은 잔여량 10%입니다. 명령은 에이전트별로 수정한 지침 파일 경로를 출력합니다. `$HOME/.local/bin`이 `PATH`에 없다면 셸 설정에 직접 추가하세요.

릴리스 준비에는 `scripts/build-release.sh 0.2.0`을 사용합니다. 6개 플랫폼 아카이브와 실제 SHA-256 체크섬을 `dist/`에 만들며, 태그 릴리스 GitHub Actions 워크플로가 이를 게시합니다.

별도 tap 체크아웃에서 릴리스 체크섬 manifest로 Homebrew formula를 생성하려면 다음을 실행합니다. `--renames`는 이전 formula 이름을 새 이름으로 연결하는 `formula_renames.json`도 함께 만듭니다.

```sh
python3 scripts/generate-formula.py 0.2.0 dist/checksums.txt ../homebrew-tap/Formula/agent-usage-reserve.rb --renames ../homebrew-tap/formula_renames.json
```

생성기는 6개 릴리스 아카이브의 실제 체크섬을 모두 요구하고 macOS/Linux용 실제 URL과 해시를 씁니다. 생성된 formula는 별도 tap 저장소에서 검토한 뒤 게시하세요.

데모 GIF와 트랜스크립트를 다시 만들려면 바이너리를 빌드한 뒤 스크립트를 실행합니다(Pillow가 있는 Python과 고정폭 글꼴이 필요합니다).

```sh
go build -o /tmp/agent-usage-reserve ./cmd/agent-usage-reserve
AGENT_USAGE_RESERVE_DEMO_BINARY=/tmp/agent-usage-reserve python3 scripts/generate-demo.py
```
