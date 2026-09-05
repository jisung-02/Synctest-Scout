# 외부 자료 고지

[English](THIRD_PARTY_NOTICES.md) | [한국어](THIRD_PARTY_NOTICES.ko.md)

Synctest Scout 원본 코드는 [MIT 라이선스](LICENSE)로 제공됩니다. CLI는 외부 Go 라이브러리에 의존하지 않습니다. 라이선스 원문은 영어 파일을 기준으로 합니다.

## go-retry에서 파생된 실험 패치

`research/patches/`에는 [sethvargo/go-retry](https://github.com/sethvargo/go-retry)의 커밋 `f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502`에서 가져온 소스와 수정된 테스트 코드가 포함됩니다.

해당 소스는 Apache-2.0이며 [원문 사본](research/licenses/go-retry-APACHE-2.0.txt)을 보존합니다. 패치는 선택한 테스트에 `testing/synctest` 래퍼와 동기화 호출을 추가한 것으로 원본 그대로인 파일이 아닙니다.

실험은 cenkalti/backoff와 avast/retry-go의 고정 커밋도 읽습니다. 해당 체크아웃은 추적 대상 소스 바깥에 캐시되며 이 저장소나 CLI 릴리스 파일에 배포하지 않습니다.
