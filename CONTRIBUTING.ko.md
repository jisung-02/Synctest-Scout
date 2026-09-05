# Synctest Scout 기여 안내

[English](CONTRIBUTING.md) | [한국어](CONTRIBUTING.ko.md)

버그 제보, 재현 가능한 전환 사례, 문서 개선과 범위가 명확한 코드 변경을 환영합니다.

## 시작하기 전

기존 [이슈](https://github.com/jisung-02/Synctest-Scout/issues)와 PR을 검색하세요. 큰 API·CLI·분석 변경은 구현 전에 이슈로 논의하세요. [행동 강령](CODE_OF_CONDUCT.ko.md)을 따르고, 취약점은 공개 이슈 대신 [보안 정책](SECURITY.ko.md)에 따라 제보하세요.

## 개발 환경

Go 1.25 이상, Git, Python 3.12 이상이 필요합니다. 테스트에 외부 Go·Python 라이브러리는 필요 없습니다.

```sh
git clone https://github.com/jisung-02/Synctest-Scout.git
cd Synctest-Scout
go tool synctest-scout help
make build
```

Windows에서는 Make 대신 Python 명령을 직접 실행하세요.

## 제출 전 검증

```sh
python3 scripts/quality.py lint
python3 scripts/quality.py test
python3 scripts/quality.py coverage --snapshot
```

파서나 소스 전환을 변경하면 다음 검사도 실행하세요.

```sh
python3 scripts/quality.py stress
python3 scripts/quality.py fuzz --fuzztime=60s
```

CI는 전체 statement coverage 95%, 각 Go 패키지 90%를 요구합니다. 기준을 맞추려고 코드를 제외하거나 검증을 약화하지 마세요. 실제 회귀를 구별하는 테스트를 추가하세요. 전체 검사와 아티팩트는 [테스트 안내](docs/testing.ko.md)를 참고하세요.

선택적 공개 저장소 실험은 `python3 research/experiment.py`로 실행합니다. 외부 코드를 내려받아 임시 사본에서 테스트하며 일반 CI와 별개입니다.

## 코드와 문서 원칙

- 제안에 명시하지 않았다면 최소 Go 버전 1.25를 유지합니다.
- Go 코드는 `gofmt`를 적용합니다. CLI 출력과 기본 문서는 영어로 작성합니다. 한국어 문서는 같은 이름의 `.ko.md` 파일로 두고 상단에 양방향 링크를 넣습니다. 동작 변경 시 두 언어를 함께 갱신합니다.
- 패키지 패턴, `-C`, build tag, `fix -diff` 종료 코드 등 Go 도구와 익숙한 동작을 유지합니다.
- 소스 보존, 수정 범위와 진단을 정확성 요건으로 취급합니다. 미리보기는 파일을 수정하면 안 됩니다.
- 전환기 수정 시 import 별칭, 이름 가림, subtest, 주석과 반복 실행을 검증합니다.
- 문법상 후보와 안전성이 입증된 변환을 구별합니다. 보장 범위를 조용히 넓히지 마세요.
- 일반 테스트는 자체 fixture로 실행하고 외부 네트워크나 저장소 다운로드를 추가하지 않습니다.

## Pull request

각 PR은 한 가지 문제에 집중하고 문제, 변경 후 동작과 실행한 검사를 설명하세요. 버그 수정에는 최소 재현 사례나 회귀 테스트를 포함하세요. 사용자 동작이 바뀌면 README, 명령 도움말과 변경 이력을 갱신하고, Go 변경 시 커버리지 스냅샷을 갱신하세요.

기여한 원본 코드는 프로젝트의 [MIT 라이선스](LICENSE)로 제공하는 데 동의한 것으로 봅니다. 외부 자료의 출처와 라이선스를 보존하세요.
