# Провайдер Domain Manager (DM)

Провайдер **Domain Manager (DM)** интегрирует управление SSL-сертификатами и доменами Clickadu в Kubernetes. Он автоматизирует процесс получения сертификатов, объединяя их в Fullchain и заказывая новые сертификаты по требованию.

## Основные возможности
- **Авто-провижининг**: Автоматический заказ RSA/ECDSA сертификатов. Поддерживается создание по имени домена или по его ID в системе DM.
- **Поддержка субдоменов (SAN)**: В режиме `dataFrom` можно указать список субдоменов через запятую в поле `property`.
- **Интеллектуальный Fullchain**: Поля `cert` и `tls.crt` всегда возвращают объединенную цепочку (Certificate + CA).
- **Списки живых доменов**: Позволяет получить список имен активных доменов (статусы 40-57) через запятую.

---

## Настройка SecretStore

Для работы нужен API токен, сохраненный в секрете Kubernetes.

```yaml
apiVersion: external-secrets.io/v1
kind: ClusterSecretStore
metadata:
  name: domain-manager
spec:
  provider:
    dm:
      baseURL: "https://dm.example.com"
      auth:
        secretRef:
          apiToken:
            name: dm-api-token
            key: token
            namespace: external-secrets
```

---

## Примеры использования

### 1. Таргетированные запросы (data)
Используется для получения конкретных полей в заданные ключи.

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: dm-targeted
spec:
  refreshInterval: "1h"
  secretStoreRef:
    name: domain-manager
    kind: ClusterSecretStore
  target:
    name: cert-data
  data:
    # Получение Fullchain (Cert + CA) по домену
    - secretKey: fullchain.pem
      remoteRef:
        key: rsa/domain/example.com
        property: cert

    # Получение только приватного ключа
    - secretKey: private.key
      remoteRef:
        key: rsa/domain/example.com
        property: key

    # Получение только CA бандла (алиас ca.crt также работает)
    - secretKey: root.ca
      remoteRef:
        key: rsa/domain/example.com
        property: ca

    # Получение сертификата по ID сертификата
    - secretKey: cert-by-id
      remoteRef:
        key: rsa/id/123
        property: cert

    # Получение имени домена по ID домена
    - secretKey: domain-name
      remoteRef:
        key: domain/id/42
```

### 2. Автоматизация и SAN (dataFrom)
Используется для создания готовых TLS секретов и массовой загрузки.

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: dm-automated
spec:
  refreshInterval: "1h"
  secretStoreRef:
    name: domain-manager
    kind: ClusterSecretStore
  target:
    name: example-tls
    template:
      type: kubernetes.io/tls
  dataFrom:
    # Авто-выпуск RSA сертификата для: test.com, www.test.com, api.test.com
    # В секрете создадутся ключи: tls.crt (fullchain) и tls.key
    - extract:
        key: rsa/domain/test.com
        property: www,api

    # То же самое, но поиск домена по его ID в системе DM
    - extract:
        key: rsa/domain_id/42
        property: www,api

    # Получение списка всех "живых" доменов (40-57) группы 232 через запятую
    # Создаст ключ 'domains' со значением "a.com,b.com,c.com"
    - extract:
        key: domain/type_id/232
```

---

## Таблица свойств (Property)

| Свойство | Результат | Комментарий |
|----------|-----------|-------------|
| `cert` / `tls.crt` | **Fullchain** | Certificate + CA Bundle |
| `key` / `tls.key` | Private Key | Тело приватного ключа |
| `ca` / `ca.crt` | **CA Only** | Только корневой/промежуточный сертификат |
| *любое другое* | - | Список субдоменов (только для `domain/` и `domain_id/` ключей) |

## Форматы ключей (Key)
* `rsa/domain/<host>` — Поиск/создание RSA по имени домена.
* `ecdsa/domain/<host>` — Поиск/создание ECDSA по имени домена.
* `rsa/domain_id/<id>` — Поиск/создание RSA по ID домена.
* `rsa/id/<id>` — Поиск RSA по ID сертификата.
* `domain/id/<id>` — Имя домена по его ID.
* `domain/type_id/<id>` — Список доменов группы.
```
