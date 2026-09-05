# 초안 — 제출하지 않음

[English](PR_DRAFT.md) | [한국어](PR_DRAFT.ko.md)

대상: sethvargo/go-retry 커밋 f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502.

제안 제목: Use synctest for backoff expiry and context timeout tests

Backoff 만료 테스트는 실행마다 250ms를 기다리고 context 테스트 두 개는 50ms와 10ms deadline을 기다립니다. 본문을 `synctest.Test`로 감싸 가상 시간으로 타이머를 진행하되 첫 `t.Parallel`은 bubble 밖에 유지합니다. Backoff 테스트의 명시적 sleep 뒤에는 `synctest.Wait`를 추가합니다. 모듈은 이미 Go 1.25를 선언합니다.

Go 1.27.1을 실행하는 macOS ARM64 한 대에서 컴파일을 제외하고 프로세스 시작을 포함한 20회 backoff 배치 중앙값이 5.050초에서 0.0126초로 줄었습니다. 변경 전후 전체 테스트가 `-race -count=10`을 통과했고 선택한 각 테스트는 각 버전에서 60번 통과했습니다. 제품 코드의 만료 조건 `<= 0`을 `< 0`으로 바꾸자 변환한 테스트가 정확한 만료 경계에서 실패했습니다.

검토 사항: `deadline_exceeded`는 이제 가상 경과 시간을 측정합니다. Deadline과 backoff 관계를 검증하지만 실제 스케줄러 지연을 측정하지 않습니다. 실제 시간 검증이 의도된 성능 계약이라면 이 변경을 제외하거나 분리하세요.

첨부 로컬 패치:

- [Backoff 만료](patches/go-retry-backoff.patch)
- [Context timeout](patches/go-retry-context.patch)

전체 방법과 한계는 [실험 보고서](REPORT.ko.md)를 참고하세요.
