<!--Описание-->
# Оглавление

[Вводная часть](#Вводная-часть)

[Установка](#Установка)

[Сборка](#Сборка)

[Описание запросов](#Запросы)

[Диаграммы запросов](#Диаграммы)

[Пример curl запросов](#Пример)

# Вводная часть
Проект состоит из двух частей: Storage и Computation серверов. На Storage приходят запросы о добавлении математической задачи, он отслеживает серверы вычислений, контроллирует вычисление. На Computation приходит только число и операция, в ответ приходит число.

<!--Установка-->
# Установка
```git clone https://github.com/XJIeI5/calculator.git```

```go get .```


<!--Запуск-->
# Сборка

### Storage сервер
```
cd cmd/storage
go build server.go
start server.exe --port=3000
cd ../../
```

__флаги__:
- host: хост сервера, по умолчанию "http://localhost"
- port: порт сервера, по умолчанию 8080

### Computation сервер
```
cd cmd/compute 
go build server.go
start server.exe --pc=5 --port=5000
cd ../../
```

__флаги__:
- host: хост сервера, по умолчанию "http://localhost"
- port: порт сервера, по умолчанию 8080
- pc: parallel computations, количество запущенных горутин, по умолчанию 10

<!--Запросы-->
# Запросы
### Storage сервер

- /regist_compute

> POST-запрос, ContentType application/json
> 
> тело запроса: json {"addr": "*адрес сервера вычислений*"}
> 
> возвращает статус-код

> чтобы сервер вообще мог послать запрос на вычисление, надо зарегестрировать сервер вычислений. но делать это надо не через этот запрос, а через запрос на регистрацию самого вычислительного сервера

> не следует делать этот запрос для регистарции сервера вычислений

- /add_expr

> POST-запрос, ContentType application/json
> 
> тело запроса: json {"expr": "*выражение*"}
> 
> возвращает id выражения (число)

> возвращает id выражения, по запросу /get_result можно получить результат

> `curl -L "http://localhost:8080/add_expr" -H "Content-Type: application/json" -d "{\"expr\": \"10 * (2 + 1)\"}"`

- /get_result
  
> GET-запрос
> 
> url-query: ?id=*id выражения*
> 
> возвращает json {"state": "*состояние вычисление*", "result": "*ответ*"}

> возвращает состояние вычисления и его результат

> `curl -L "http://localhost:8080/get_result?id=2146560825"`

- /set_timeout
  
> POST-запрос, ContentType application/json
> 
> тело запроса: json {"timeout": {"*символ операции*": *время в милисекундах*}
> 
> возвращает статус-код

> задает время выполнения различных операций в милисекундах. перезаписывает указанные в теле запроса, оставляет без изменений неуказанные. чтобы изменить время ожидания heartbeat'а от сервера вычислений, *символ операции* должен быть "__wait"

> `curl -L "http://localhost:3000/set_timeout" -H "Content-Type: application/json" -d "{\"timeout\": {\"+\": 10000}}"`

- /heart
  
> GET-запрос
> 
> возвращает статус-код

> обновляет время последнего пинга от сервера вычислений, который прислал запрос. если время, которое сервер вычислений не присылал пинг, больше пяти секунд, он считается недоступным.
> если не присылал больше времени, определяемого "__wait" в /set_timeout запросе, сервер вычислений удаляется из списка доступных и его надо заново регистрировать

> не следует делать этот запрос для пинга

- /get_compute

> GET-запрос
> 
> возвращает json [{"addr": "*адрес сервера вычислений*", "state": "*состояние*", "last_beat": "*время последнего пинга*"}, ...]

> возвращает сервера вычислений и их состояние

> `curl -L "http://localhost:3000/get_compute"`

### Computation сервер

- /regist

> POST-запрос, ContentType application/json
>
> тело запроса: json {"addr": "*адрес сервера хранения*"}
>
> возвращает статус-код

> этот запрос регистрирует сервер вычислений для сервера хранения. теперь этот сервер может быть задействован для вычислений

> `curl -L "http://localhost:5000/regist" -H "Content-Type: application/json" -d "{\"addr\": \"http://localhost:3000\"}"`

- /exec

> POST-запрос, ContentType application/json
>
> тело запроса: json {"op_info": {"a": *первое число*, "b": *второе число*, "op": "*символ операции*"}, "duration": *количество времени в миллисекундах для выполнения операции*}
>
> возвращает число

> запрос для подсчета операции. пока поддерживаются только бинарные ( с двумя числами )

> `curl -L "http://localhost:5000/exec" -H "Content-Type: application/json" -d "{\"op_info\": {\"a\": 10, \"b\": 0.5, \"op\": \"*\"}, \"duration\": 500}"`

- /free_process

> GET-запрос
>
> возвращает число

> возвращает количество незанятых процессов, которые можно использовать для параллельного вычисления на этом сервере

> `curl -L "http://localhost:5000/free_process"`

# Диаграммы
### Регистрация сервера вычислений

```mermaid
sequenceDiagram
participant S as Storage
participant C as Compute
participant U as User

U->>C: /regist
Note over U,C: json AddressInfo
C->>S: /regist_compute
Note over C,S: json AddressInfo

loop Каждую секунду
C->>S: /heart
end
```
```mermaid
classDiagram
class AddressInfo {
  <<Информация о адрессе сервера>>
  addr: string
}
```

### Добавление выражения

```mermaid
sequenceDiagram
participant U as User
participant S as Storage
participant C as Compute

U->>S: /add_expr
activate S
S->>U: id выражения
deactivate S

loop Пока остаются непосчитанные выражения
S->>C: /exec
activate C
C->>S: результат /exec
deactivate C
end
```

### Получение выражения

```mermaid
sequenceDiagram
participant U as User
participant S as Storage

U->>S: /get_result
Note over S,U: ?id=123
activate S
S->>U: json по id выражения
Note over S,U: json ExpressionState
deactivate S
```
```mermaid
classDiagram
class ExpressionState {
  <<возвращаемое значение /get_result>>
  state:  string
  result: float, string
}
```

### Задача значений таймаута

```mermaid
sequenceDiagram
participant U as User
participant S as Storage

U->>S: /set_timeout
Note over U,S: json Timeout
```
```mermaid
classDiagram
class Timeout {
  <<Список таймаутов для операндов математических операций>>
  timeout: список
}
class OperandTimeout {
  <<Таймаут для одного отдельного операнда>>
  operandSymbol: string
  duration:           int
}
Timeout "1" --> "n" OperandTimeout
```

### Получение списка воркеров

```mermaid
sequenceDiagram
participant U as User
participant S as Storage

U->>S: /get_compute
activate S
S->>U: список ComputeState
deactivate S
```
```mermaid
classDiagram
direction LR
class ComputeState {
  <<состояние одного Compute>>
  addr:      string
  state:     string
  lastBeat: time
}
```

### Получение свободных вычислительных мощностей сервера вычислений

```mermaid
sequenceDiagram
participant S as Storage
participant C as Compute

S->>C: /regist
activate C
C->>S: количество незанятых в вычислениях горутин
deactivate C
```

# Пример

```
start cmd/compute/server.exe --port=5000

start cmd/compute/server.exe --port=6000

start cmd/storage/server.exe --port=3000

curl -L 'http://localhost:5000/regist' -H 'Content-Type: application/json' -d '{"addr": "http://localhost:3000"}'

curl -L 'http://localhost:6000/regist' -H 'Content-Type: application/json' -d '{"addr": "http://localhost:3000"}'

curl -L 'http://localhost:3000/set_timeout' -H 'Content-Type: application/json' -d '{"timeout": {"+": 10000}}'

curl -L 'http://localhost:3000/add_expr' -H 'Content-Type: application/json' -d '{"expr": "10 * (2 + 1)"}'

curl -L 'http://localhost:3000/get_result?id=2146560825'

curl -L 'http://localhost:3000/add_expr' -H 'Content-Type: application/json' -d '{"expr": "30 + 0.5"}'

curl -L 'http://localhost:3000/get_result?id=2146560825'
```

