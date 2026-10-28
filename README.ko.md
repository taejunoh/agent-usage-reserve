# Codex Usage Reserve

`codex-usage-reserve`는 Codex 전역 지침에 사용량 여유분 정책을 추가하고 관리하는 비공식 로컬 도구입니다. 서비스, 텔레메트리, API 키 접근, 자격 증명 수집, 백그라운드 폴링이 없습니다.

## 현재 체크아웃에서 빌드하고 활성화하기

소스 빌드에는 Go 1.23 이상이 필요합니다.

```sh
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/codex-usage-reserve" ./cmd/codex-usage-reserve
"$HOME/.local/bin/codex-usage-reserve" enable
```

기본 기준은 잔여량 10%입니다. 명령은 수정한 지침 파일 경로를 출력합니다. `$HOME/.local/bin`이 `PATH`에 없다면 셸 설정에 직접 추가하세요.

## 릴리스 설치

아직 GitHub 릴리스와 Homebrew 탭은 공개되지 않았습니다. 첫 릴리스 이후 직접 설치 명령은 다음과 같습니다.

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.sh | sh
```

Windows PowerShell 설치는 첫 릴리스 이후 다음과 같습니다.

```powershell
irm https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.ps1 | iex
```

설치기는 최신 릴리스 태그를 한 번 조회하고 두 자산 다운로드를 그 태그에 고정합니다. SHA-256과 실행 파일 버전을 확인한 뒤 사용자 디렉터리에 설치하고 `enable`을 실행합니다. `sudo`를 사용하거나 셸 시작 파일을 수정하지 않습니다. Unix 기본 경로는 `~/.local/bin`, Windows는 `%LOCALAPPDATA%/Programs/codex-usage-reserve/bin`입니다. Windows에서는 출력된 디렉터리를 사용자 `PATH`에 추가하세요. 현재 PowerShell 세션에는 출력된 `$env:Path = "...;$env:Path"` 명령을 실행하면 됩니다.

첫 릴리스 및 별도 `homebrew-tap` 저장소 공개 후 Homebrew 설치 명령은 다음과 같습니다.

```sh
brew tap taejunoh/tap
brew install taejunoh/tap/codex-usage-reserve
codex-usage-reserve enable
```

Homebrew 설치는 실행 파일만 설치하며 정책 활성화는 별도로 실행합니다.

## 업데이트

릴리스가 공개되면 직접 설치기를 다시 실행해 업데이트할 수 있습니다. 설치기는 기준값 없이 `enable`을 실행하므로 기존 사용자 지정 기준을 유지합니다.

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/codex-usage-reserve/main/install.sh | sh
```

Homebrew는 탭과 formula를 업데이트한 뒤 정책 상태를 확인하고, 비활성화되어 있으면 다시 활성화하세요.

```sh
brew update
brew upgrade taejunoh/tap/codex-usage-reserve
codex-usage-reserve status
codex-usage-reserve enable  # status가 비활성 상태일 때 실행; 현재 기준 유지
```

## 명령

```text
codex-usage-reserve enable [--threshold 10] [--codex-home PATH]
codex-usage-reserve set 15 [--codex-home PATH]
codex-usage-reserve status [--codex-home PATH]
codex-usage-reserve disable [--codex-home PATH]
codex-usage-reserve --version
```

기준은 1–99 사이의 정수 퍼센트입니다. `enable`을 기준값 없이 반복하면 현재 사용자 설정을 유지합니다. `set`은 활성화된 정책을 변경합니다. `status`는 관리 정책과 대상 파일을 보여주며 계정 사용량이나 모델 동작을 확인하지 않습니다.

도구는 Codex 홈의 전역 `AGENTS.override.md` 또는 `AGENTS.md` 중 유효한 파일에서 표시된 정책 블록만 변경합니다. 변경 전에 원본을 같은 디렉터리의 비공개 백업 파일로 저장합니다. `disable`은 다른 텍스트를 보존하면서 관리 블록을 제거합니다. 파일을 바꾸기 전에 작업 중 대상 파일이 변경됐는지 확인해 많은 외부 편집을 감지하지만, 별도 편집기를 직렬화하거나 모든 동시 쓰기를 반드시 감지하지는 못합니다. 정책 비활성화와 실행 파일 삭제는 별도 작업입니다. `disable` 후 바이너리를 삭제하거나 `brew uninstall taejunoh/tap/codex-usage-reserve`를 실행하세요.

### 백업 복구

관리 블록 제거는 먼저 `codex-usage-reserve disable`을 사용하세요. 백업 파일 이름은 `.codex-usage-reserve-backup-*`이며 Codex 홈 디렉터리에 있습니다. macOS/Linux에서 확인하려면:

```sh
codex_home=${CODEX_HOME:-"$HOME/.codex"}
find "$codex_home" -type f -name '.codex-usage-reserve-backup-*' -print
```

선택한 백업과 현재 대상 파일을 먼저 살펴보세요. 전체 파일을 복원하면 백업 이후의 편집 내용이 사라질 수 있습니다. 의도적으로 전체 복원하기 전 현재 대상 파일도 복사해 두세요. 대상은 `status`가 출력한 실제 파일(`AGENTS.md` 또는 `AGENTS.override.md`)을 선택합니다.

```sh
backup='/full/path/to/.codex-usage-reserve-backup-...'
target="$codex_home/AGENTS.md" # AGENTS.override.md가 활성 파일이면 그 경로 사용
diff -u "$target" "$backup" || true
cp -p "$target" "$target.before-restore"
cp -p "$backup" "$target" # 전체 파일 복원을 의도한 경우에만 실행
```

Windows PowerShell에서 백업을 확인하려면:

```powershell
$codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME '.codex' }
Get-ChildItem -Force -File $codexHome -Filter '.codex-usage-reserve-backup-*' |
  Sort-Object LastWriteTime -Descending
```

```powershell
$backup = 'C:\full\path\to\.codex-usage-reserve-backup-...'
$target = Join-Path $codexHome 'AGENTS.md' # AGENTS.override.md가 활성 파일이면 그 경로 사용
Get-FileHash -LiteralPath $backup, $target
Copy-Item -LiteralPath $target -Destination "$target.before-restore"
Copy-Item -LiteralPath $backup -Destination $target -Force # 전체 파일 복원을 의도한 경우에만 실행
```

나중에 추가한 내용을 보존하려면 필요한 부분만 골라 직접 반영하세요.

## 적용 범위와 제한

정책은 일반 작업 시작 전과 주요 단계 사이에 사용할 수 있는 계정 사용량 제한을 확인하도록 Codex에 안내합니다. 실제 예약 작업 실행은 예외입니다. 명시적인 계속 요청은 지정한 작업에만 적용하며, 작업을 지정하지 않으면 현재 작업에만 적용됩니다. 사용자가 임시 기준을 지정하면 그 값을 우선합니다.

이는 행동 지침이며 계정 사용량을 강제로 제한하지 않습니다. 현재 Codex 데스크톱이나 작업 환경에서 제공되는 사용량 데이터와 도구에만 의존하므로 모든 CLI 또는 예약 실행에서 사용할 수 있다고 보장하지 않습니다. 데이터가 없으면 추측하지 않고 알립니다. 사용자의 모델 선택·위임 정책은 포함하지 않습니다.

## 릴리스 자산과 탭 준비

릴리스 태그를 만든 뒤 `scripts/build-release.sh 0.1.0`을 실행하면 6개 플랫폼 아카이브와 실제 SHA-256 체크섬을 `dist/`에 생성합니다. 태그 릴리스 GitHub Actions 워크플로가 이 파일로 릴리스를 게시합니다.

릴리스의 `checksums.txt`를 받은 뒤 별도 탭 체크아웃에 Homebrew formula를 생성합니다.

```sh
python3 scripts/generate-formula.py 0.1.0 dist/checksums.txt ../homebrew-tap/Formula/codex-usage-reserve.rb
```

생성기는 6개 릴리스 아카이브의 실제 체크섬을 모두 요구하고 macOS/Linux용 실제 URL과 해시를 씁니다. formula를 탭 저장소에 검토·게시한 뒤에만 Homebrew 설치가 가능하다고 안내하세요.
