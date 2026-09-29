# Changelog

## [0.4.2](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.4.1...tfclassify-plugin-sdk-v0.4.2) (2026-09-29)


### Bug Fixes

* **ci:** bump Go to 1.25.14, pin govulncheck, sync generated protobuf headers ([6bd7ec4](https://github.com/jokarl/tfclassify/commit/6bd7ec4c8697ebbbfa9c6e957a6d394c252dae02))


### Dependencies

* **deps:** bump github.com/gobwas/glob from 0.2.3 to 1.0.0 ([#229](https://github.com/jokarl/tfclassify/issues/229)) ([b525a37](https://github.com/jokarl/tfclassify/commit/b525a3798e44aaddf990220ea77037f49335939a))
* **deps:** bump github.com/hashicorp/go-plugin from 1.7.0 to 1.8.0 ([#180](https://github.com/jokarl/tfclassify/issues/180)) ([7644dcd](https://github.com/jokarl/tfclassify/commit/7644dcdf73ef6808dbced9d9ee398d323cdf25ba))
* **deps:** bump google.golang.org/grpc from 1.80.0 to 1.81.0 in /sdk ([#184](https://github.com/jokarl/tfclassify/issues/184)) ([90a886e](https://github.com/jokarl/tfclassify/commit/90a886e9d86a12a899eacb4344cae22f9ebe5698))
* **deps:** bump google.golang.org/grpc from 1.81.0 to 1.81.1 ([#189](https://github.com/jokarl/tfclassify/issues/189)) ([b37ff99](https://github.com/jokarl/tfclassify/commit/b37ff99ccffb1881d59008c9734ff09cd01c77c2))
* **deps:** bump google.golang.org/grpc from 1.81.0 to 1.81.1 in /sdk ([#188](https://github.com/jokarl/tfclassify/issues/188)) ([7d00dc7](https://github.com/jokarl/tfclassify/commit/7d00dc7b3e741ea19447e18724de72a1dd2d2ad6))
* **deps:** bump google.golang.org/grpc from 1.81.1 to 1.82.0 in /sdk ([#202](https://github.com/jokarl/tfclassify/issues/202)) ([19731ab](https://github.com/jokarl/tfclassify/commit/19731abba42e2802fa7b932fa7d995fe23c51d79))
* **deps:** bump google.golang.org/grpc from 1.82.0 to 1.82.1 ([#209](https://github.com/jokarl/tfclassify/issues/209)) ([b253094](https://github.com/jokarl/tfclassify/commit/b253094455f8855a9d3d9439f19781b9470a665f))
* **deps:** bump google.golang.org/grpc from 1.82.0 to 1.82.1 in /sdk ([#207](https://github.com/jokarl/tfclassify/issues/207)) ([53d99e2](https://github.com/jokarl/tfclassify/commit/53d99e2522252318d867baca4b7df1a768cb6652))
* **deps:** bump google.golang.org/grpc from 1.82.1 to 1.83.0 in /sdk ([#216](https://github.com/jokarl/tfclassify/issues/216)) ([e058b8e](https://github.com/jokarl/tfclassify/commit/e058b8eb546d5fc78d53adc04111623c77ab1298))
* **deps:** bump google.golang.org/grpc from 1.83.0 to 1.83.1 ([#223](https://github.com/jokarl/tfclassify/issues/223)) ([f2b513b](https://github.com/jokarl/tfclassify/commit/f2b513b74c761324fc9fd0f2b21040a04900e53f))
* **deps:** bump google.golang.org/grpc from 1.83.1 to 1.83.2 ([#226](https://github.com/jokarl/tfclassify/issues/226)) ([77cf1cf](https://github.com/jokarl/tfclassify/commit/77cf1cf1e97051d918b90a0124c20373642c659e))
* **deps:** bump google.golang.org/protobuf from 1.36.11 to 1.36.12 ([#219](https://github.com/jokarl/tfclassify/issues/219)) ([69e4444](https://github.com/jokarl/tfclassify/commit/69e44440edd404b999a5048255b1e53658c0db29))

## [0.4.1](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.4.0...tfclassify-plugin-sdk-v0.4.1) (2026-04-21)


### Dependencies

* **deps:** bump github.com/hashicorp/go-version from 1.8.0 to 1.9.0 ([#163](https://github.com/jokarl/tfclassify/issues/163)) ([d368a0e](https://github.com/jokarl/tfclassify/commit/d368a0e5c928ed948e82364d3a0222bc1b7d2627))
* **deps:** bump google.golang.org/grpc from 1.79.3 to 1.80.0 in /sdk ([#162](https://github.com/jokarl/tfclassify/issues/162)) ([0014dd9](https://github.com/jokarl/tfclassify/commit/0014dd9f369bdf7e891abe994025d812b5396e4e))

## [0.4.0](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.3.0...tfclassify-plugin-sdk-v0.4.0) (2026-03-25)


### Features

* add ignore_attributes to exclude cosmetic changes from classifi… ([#138](https://github.com/jokarl/tfclassify/issues/138)) ([85af62c](https://github.com/jokarl/tfclassify/commit/85af62c97a80f880bac749c87bdb809d5b804ad2))


### Dependencies

* **deps:** bump google.golang.org/grpc from 1.79.1 to 1.79.3 in /sdk ([#136](https://github.com/jokarl/tfclassify/issues/136)) ([e90c7fc](https://github.com/jokarl/tfclassify/commit/e90c7fc91df295476f0e708cc2a1e9a475bd19ce))

## [0.3.0](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.2.1...tfclassify-plugin-sdk-v0.3.0) (2026-02-26)


### Features

* add module-scoped rules, drift classification, and topology analysis ([#65](https://github.com/jokarl/tfclassify/issues/65)) ([87b33bd](https://github.com/jokarl/tfclassify/commit/87b33bd3a78750abdec93d3a1758b897f6a88dee))

## [0.2.1](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.2.0...tfclassify-plugin-sdk-v0.2.1) (2026-02-22)


### Bug Fixes

* address v0.5.0 critique findings ([#42](https://github.com/jokarl/tfclassify/issues/42)) ([74abb72](https://github.com/jokarl/tfclassify/commit/74abb72dc035d0dda0c3fc93b1970b154330f974))

## [0.2.0](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.1.0...tfclassify-plugin-sdk-v0.2.0) (2026-02-17)


### Features

* classification-scoped plugin config and --detailed-exitcode flag ([#14](https://github.com/jokarl/tfclassify/issues/14)) ([128646b](https://github.com/jokarl/tfclassify/commit/128646b8d33bb64cf585bc3d9460bee98a96d924))

## [0.1.0](https://github.com/jokarl/tfclassify/compare/tfclassify-plugin-sdk-v0.0.1...tfclassify-plugin-sdk-v0.1.0) (2026-02-14)


### Features

* implement tfclassify MVP with plugin architecture, Azure deep inspection, and CI/CD ([#2](https://github.com/jokarl/tfclassify/issues/2)) ([8d15cd6](https://github.com/jokarl/tfclassify/commit/8d15cd65748b2465b96ff1af93fce90598dcf84b))


### Bug Fixes

* bump Go to 1.26.0 to resolve 16 stdlib vulnerabilities ([d02e40e](https://github.com/jokarl/tfclassify/commit/d02e40e690ce0cd000054108852568251809c377))
