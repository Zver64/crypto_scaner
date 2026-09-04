# Changelog

## [0.6.0](https://github.com/Zver64/crypto_scaner/compare/v0.5.0...v0.6.0) (2026-09-04)


### Features

* show detail price history ([22a083b](https://github.com/Zver64/crypto_scaner/commit/22a083b22c01a3df9ceee5a3029ff5a63a694130))
* telegram bot access management for the Administrator ([3006322](https://github.com/Zver64/crypto_scaner/commit/300632204530a82b6284edd239b46c5f38422e28)), closes [#24](https://github.com/Zver64/crypto_scaner/issues/24)

## [0.5.0](https://github.com/Zver64/crypto_scaner/compare/v0.4.0...v0.5.0) (2026-09-04)


### Features

* show grid step recommendations ([3a329ea](https://github.com/Zver64/crypto_scaner/commit/3a329ea6f5cce877c1b8b8bff9b3a94759951dd9))


### Bug Fixes

* remove redundant text ([ff964b8](https://github.com/Zver64/crypto_scaner/commit/ff964b81f88c93514d41f93860927693592daa60))

## [0.4.0](https://github.com/Zver64/crypto_scaner/compare/v0.3.0...v0.4.0) (2026-09-03)


### Features

* add seven-day price charts to analysis results ([#19](https://github.com/Zver64/crypto_scaner/issues/19)) ([0b24d6c](https://github.com/Zver64/crypto_scaner/commit/0b24d6c41fb9da540bc6b7bcd4a55a790aa5f6a9))
* Show instrument range statistics and sample coverage ([365f8ca](https://github.com/Zver64/crypto_scaner/commit/365f8ca401b4cd748bd13774a6bd61c8681589d9))
* tweaket default filter ([09f1c4e](https://github.com/Zver64/crypto_scaner/commit/09f1c4ec1585a85252b8810d1c0753e17647ab70))


### Bug Fixes

* refine responsive scan interface ([cafe41c](https://github.com/Zver64/crypto_scaner/commit/cafe41c7f4edb7943821b3f86e008174dc2ca2a7))
* show units only in scan field labels ([be76167](https://github.com/Zver64/crypto_scaner/commit/be761678fe47b588bee45b1e88e42b6d114d139f))

## [0.3.0](https://github.com/Zver64/crypto_scaner/compare/v0.2.0...v0.3.0) (2026-09-02)


### Features

* add administrator bootstrap command ([dee408a](https://github.com/Zver64/crypto_scaner/commit/dee408afc48517ff1ebd4ab397ae28efc534bad2))
* add average entry price calculators ([4faff51](https://github.com/Zver64/crypto_scaner/commit/4faff51887aa77775e8af6fc0e49b96e390bddeb))
* add candle range percentile analyzer ([1159e7d](https://github.com/Zver64/crypto_scaner/commit/1159e7dce70f4a0b89bf338dda8220542047690f))
* add COIN-M average entry validation ([2db8cdb](https://github.com/Zver64/crypto_scaner/commit/2db8cdb9e9cf26c4baa79d525556d872da7f39bb))
* add COIN-M liquidation calculator ([6a592bf](https://github.com/Zver64/crypto_scaner/commit/6a592bf89737e0b36a509c4a79becaab8ea55dd4))
* add composable analysis criteria ([31434f3](https://github.com/Zver64/crypto_scaner/commit/31434f30e527d481ba34535aa52e473349e8bd0c))
* add conservative liquidation calculator ([518da42](https://github.com/Zver64/crypto_scaner/commit/518da42a0b061a21c0ee711c650534c30e36757b))
* add explicit PostgreSQL migrations ([84dd2a7](https://github.com/Zver64/crypto_scaner/commit/84dd2a7fd340153a4798692240914bc79d278da6))
* add hourly market analysis ([c4005e7](https://github.com/Zver64/crypto_scaner/commit/c4005e78bd8d3e6760cf77bfceeb569b76098e6b))
* add incremental market synchronization ([d6ea3a2](https://github.com/Zver64/crypto_scaner/commit/d6ea3a24118a93c8e7d92577e30a1db39b29a76f))
* add keyed volatility criterion instances ([#2](https://github.com/Zver64/crypto_scaner/issues/2)) ([dabcb52](https://github.com/Zver64/crypto_scaner/commit/dabcb5271877b500e7c8526d7e40d4a6324bcbc7))
* add local Telegram auth workflow ([31b3364](https://github.com/Zver64/crypto_scaner/commit/31b336428f3cc98c1cef23f62905cab4a86bc4ac))
* add market cap analysis ([7aad533](https://github.com/Zver64/crypto_scaner/commit/7aad53337bacfaf8f21044d7801751d277b37e33))
* add persisted Market Scan result sorting ([0712d0c](https://github.com/Zver64/crypto_scaner/commit/0712d0c22b1d188017826c171df4ff37741cc1b1))
* add persisted Market Scan result sorting ([#5](https://github.com/Zver64/crypto_scaner/issues/5)) ([ba6efbf](https://github.com/Zver64/crypto_scaner/commit/ba6efbf3cb7bdaa5f83c22c9b5c14bf5c5a6facc))
* add PostgreSQL stores and readiness ([3e1f642](https://github.com/Zver64/crypto_scaner/commit/3e1f64261487af089a47a435d851e98bd34286c6))
* add runtime lifecycle and liveness ([96ae993](https://github.com/Zver64/crypto_scaner/commit/96ae9932c90d1ea96575669c2ae8c720e0b4f76c))
* add USD-M average entry calculator ([f927825](https://github.com/Zver64/crypto_scaner/commit/f927825de37383995ad363b22575a1bd2015eff0))
* authenticate Telegram Mini App users ([045ed28](https://github.com/Zver64/crypto_scaner/commit/045ed2890645aa4b203e8da6ce8ece8d4eaf5e21))
* backfill closed daily candles ([8c919ac](https://github.com/Zver64/crypto_scaner/commit/8c919ac22105bb397a247cd124b68e12caa33716))
* configure local development ([f2d4a3f](https://github.com/Zver64/crypto_scaner/commit/f2d4a3fc6ad689113defd9df65475e63ff2c32ca))
* expose authenticated percentile analysis ([5a5c8c6](https://github.com/Zver64/crypto_scaner/commit/5a5c8c63267a5be1fbd6ad3ec9cf8201df71b4a9))
* **frontend:** analyze scan instruments ([ea2b7aa](https://github.com/Zver64/crypto_scaner/commit/ea2b7aa516b8f8bb7353d2f4ceb84884f9645022))
* **frontend:** bootstrap mini app shell ([7100552](https://github.com/Zver64/crypto_scaner/commit/7100552e2e2681d04445e8799179f6ce50ec281e))
* **frontend:** link scan results to Binance Spot ([b32cad0](https://github.com/Zver64/crypto_scaner/commit/b32cad0d20d28cab32aceb531522811dfe18e5db))
* **frontend:** refine scan results ([526c685](https://github.com/Zver64/crypto_scaner/commit/526c68582fa9507f7f58317c83cca6eaddf89e35))
* **frontend:** run market scan ([1ec3511](https://github.com/Zver64/crypto_scaner/commit/1ec3511819ef60fc9db6aabb09f733f9aa829bbc))
* initialize Go project foundation ([b7c60bf](https://github.com/Zver64/crypto_scaner/commit/b7c60bf093edc26e1e3ee83a93b8a54a0eb1ab8e))
* launch Mini App from Telegram webhook ([be610e2](https://github.com/Zver64/crypto_scaner/commit/be610e213593cc5bcfe5e16b8bb51d0037f14519))
* open Binance links through Telegram ([076f62b](https://github.com/Zver64/crypto_scaner/commit/076f62bfb078f47be80cf57af8fc9630cf4eb28c))
* show unified pipeline in instrument analysis ([b7b7604](https://github.com/Zver64/crypto_scaner/commit/b7b7604aa018e4ded4455f5a49fceab286f1df68))
* show unified pipeline in instrument analysis ([#4](https://github.com/Zver64/crypto_scaner/issues/4)) ([e423c36](https://github.com/Zver64/crypto_scaner/commit/e423c36d34eeb2206f054d479987d1dbe7623e57))
* sync Binance instrument catalog ([8aeb04e](https://github.com/Zver64/crypto_scaner/commit/8aeb04e956895d780a0eb6ff6eda3bd33b465209))
* unify the Market Scan analysis pipeline ([#3](https://github.com/Zver64/crypto_scaner/issues/3)) ([4cf9844](https://github.com/Zver64/crypto_scaner/commit/4cf9844768a63a848ebe16a42b3a7aedb7bfd1dd))


### Bug Fixes

* analyze partial market history ([0d6c1b3](https://github.com/Zver64/crypto_scaner/commit/0d6c1b3d9090cc28cc6e25a8c8cd8f925caa0314))
* **frontend:** address MVP review ([4456e97](https://github.com/Zver64/crypto_scaner/commit/4456e9726db1ed0909b3ec07223400639674cc47))
* keep market scan sorting local ([558589d](https://github.com/Zver64/crypto_scaner/commit/558589dceac7438df01bc701aa3467435c1e5c88))
* pass release version to frontend ([6ed34dd](https://github.com/Zver64/crypto_scaner/commit/6ed34ddd9cd7518dbdd95976324cbf72bd59ae7d))
* prevent Telegram swipe gesture conflicts ([8f17cd6](https://github.com/Zver64/crypto_scaner/commit/8f17cd6ff3d4300c75a34bcda940df95362f55b9))
* repair migrations and market sync ([e5325e5](https://github.com/Zver64/crypto_scaner/commit/e5325e575a6575b5b21abe9f90af86f43836e62c))

## [0.2.0](https://github.com/Zver64/crypto_scaner/compare/crypto-scanner-v0.1.0...crypto-scanner-v0.2.0) (2026-09-02)


### Features

* add administrator bootstrap command ([dee408a](https://github.com/Zver64/crypto_scaner/commit/dee408afc48517ff1ebd4ab397ae28efc534bad2))
* add average entry price calculators ([4faff51](https://github.com/Zver64/crypto_scaner/commit/4faff51887aa77775e8af6fc0e49b96e390bddeb))
* add candle range percentile analyzer ([1159e7d](https://github.com/Zver64/crypto_scaner/commit/1159e7dce70f4a0b89bf338dda8220542047690f))
* add COIN-M average entry validation ([2db8cdb](https://github.com/Zver64/crypto_scaner/commit/2db8cdb9e9cf26c4baa79d525556d872da7f39bb))
* add COIN-M liquidation calculator ([6a592bf](https://github.com/Zver64/crypto_scaner/commit/6a592bf89737e0b36a509c4a79becaab8ea55dd4))
* add composable analysis criteria ([31434f3](https://github.com/Zver64/crypto_scaner/commit/31434f30e527d481ba34535aa52e473349e8bd0c))
* add conservative liquidation calculator ([518da42](https://github.com/Zver64/crypto_scaner/commit/518da42a0b061a21c0ee711c650534c30e36757b))
* add explicit PostgreSQL migrations ([84dd2a7](https://github.com/Zver64/crypto_scaner/commit/84dd2a7fd340153a4798692240914bc79d278da6))
* add hourly market analysis ([c4005e7](https://github.com/Zver64/crypto_scaner/commit/c4005e78bd8d3e6760cf77bfceeb569b76098e6b))
* add incremental market synchronization ([d6ea3a2](https://github.com/Zver64/crypto_scaner/commit/d6ea3a24118a93c8e7d92577e30a1db39b29a76f))
* add keyed volatility criterion instances ([#2](https://github.com/Zver64/crypto_scaner/issues/2)) ([dabcb52](https://github.com/Zver64/crypto_scaner/commit/dabcb5271877b500e7c8526d7e40d4a6324bcbc7))
* add local Telegram auth workflow ([31b3364](https://github.com/Zver64/crypto_scaner/commit/31b336428f3cc98c1cef23f62905cab4a86bc4ac))
* add market cap analysis ([7aad533](https://github.com/Zver64/crypto_scaner/commit/7aad53337bacfaf8f21044d7801751d277b37e33))
* add persisted Market Scan result sorting ([0712d0c](https://github.com/Zver64/crypto_scaner/commit/0712d0c22b1d188017826c171df4ff37741cc1b1))
* add persisted Market Scan result sorting ([#5](https://github.com/Zver64/crypto_scaner/issues/5)) ([ba6efbf](https://github.com/Zver64/crypto_scaner/commit/ba6efbf3cb7bdaa5f83c22c9b5c14bf5c5a6facc))
* add PostgreSQL stores and readiness ([3e1f642](https://github.com/Zver64/crypto_scaner/commit/3e1f64261487af089a47a435d851e98bd34286c6))
* add runtime lifecycle and liveness ([96ae993](https://github.com/Zver64/crypto_scaner/commit/96ae9932c90d1ea96575669c2ae8c720e0b4f76c))
* add USD-M average entry calculator ([f927825](https://github.com/Zver64/crypto_scaner/commit/f927825de37383995ad363b22575a1bd2015eff0))
* authenticate Telegram Mini App users ([045ed28](https://github.com/Zver64/crypto_scaner/commit/045ed2890645aa4b203e8da6ce8ece8d4eaf5e21))
* backfill closed daily candles ([8c919ac](https://github.com/Zver64/crypto_scaner/commit/8c919ac22105bb397a247cd124b68e12caa33716))
* configure local development ([f2d4a3f](https://github.com/Zver64/crypto_scaner/commit/f2d4a3fc6ad689113defd9df65475e63ff2c32ca))
* expose authenticated percentile analysis ([5a5c8c6](https://github.com/Zver64/crypto_scaner/commit/5a5c8c63267a5be1fbd6ad3ec9cf8201df71b4a9))
* **frontend:** analyze scan instruments ([ea2b7aa](https://github.com/Zver64/crypto_scaner/commit/ea2b7aa516b8f8bb7353d2f4ceb84884f9645022))
* **frontend:** bootstrap mini app shell ([7100552](https://github.com/Zver64/crypto_scaner/commit/7100552e2e2681d04445e8799179f6ce50ec281e))
* **frontend:** link scan results to Binance Spot ([b32cad0](https://github.com/Zver64/crypto_scaner/commit/b32cad0d20d28cab32aceb531522811dfe18e5db))
* **frontend:** refine scan results ([526c685](https://github.com/Zver64/crypto_scaner/commit/526c68582fa9507f7f58317c83cca6eaddf89e35))
* **frontend:** run market scan ([1ec3511](https://github.com/Zver64/crypto_scaner/commit/1ec3511819ef60fc9db6aabb09f733f9aa829bbc))
* initialize Go project foundation ([b7c60bf](https://github.com/Zver64/crypto_scaner/commit/b7c60bf093edc26e1e3ee83a93b8a54a0eb1ab8e))
* launch Mini App from Telegram webhook ([be610e2](https://github.com/Zver64/crypto_scaner/commit/be610e213593cc5bcfe5e16b8bb51d0037f14519))
* show unified pipeline in instrument analysis ([b7b7604](https://github.com/Zver64/crypto_scaner/commit/b7b7604aa018e4ded4455f5a49fceab286f1df68))
* show unified pipeline in instrument analysis ([#4](https://github.com/Zver64/crypto_scaner/issues/4)) ([e423c36](https://github.com/Zver64/crypto_scaner/commit/e423c36d34eeb2206f054d479987d1dbe7623e57))
* sync Binance instrument catalog ([8aeb04e](https://github.com/Zver64/crypto_scaner/commit/8aeb04e956895d780a0eb6ff6eda3bd33b465209))
* unify the Market Scan analysis pipeline ([#3](https://github.com/Zver64/crypto_scaner/issues/3)) ([4cf9844](https://github.com/Zver64/crypto_scaner/commit/4cf9844768a63a848ebe16a42b3a7aedb7bfd1dd))


### Bug Fixes

* analyze partial market history ([0d6c1b3](https://github.com/Zver64/crypto_scaner/commit/0d6c1b3d9090cc28cc6e25a8c8cd8f925caa0314))
* **frontend:** address MVP review ([4456e97](https://github.com/Zver64/crypto_scaner/commit/4456e9726db1ed0909b3ec07223400639674cc47))
* prevent Telegram swipe gesture conflicts ([8f17cd6](https://github.com/Zver64/crypto_scaner/commit/8f17cd6ff3d4300c75a34bcda940df95362f55b9))
* repair migrations and market sync ([e5325e5](https://github.com/Zver64/crypto_scaner/commit/e5325e575a6575b5b21abe9f90af86f43836e62c))

## Changelog

All notable changes to this project will be documented in this file.
