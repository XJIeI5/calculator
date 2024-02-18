<!--Описание-->
## Вводная часть
Проект состоит из двух частей: Storage и Computation серверов. На Storage приходят запросы о добавлении математической задачи, он отслеживает серверы вычислений, контроллирует вычисление. На Computation приходит только число и операция, в ответ приходит число.

<!--Установка-->
## Установка
```git clone https://github.com/XJIeI5/calculator.git```
```go get .```


<!--Запуск-->
# Запуск
```cd cmd```

### Storage сервер
```go run storage/server.go -port=3000```

флаги:
- host: хост сервера, по умолчанию "http://localhost"
- port: порт сервера, по умолчанию 8080

### Computation сервер
```go run compute/server.go -port=5000 -pc=20```

флаги:
- host: хост сервера, по умолчанию "http://localhost"
- port: порт сервера, по умолчанию 8080
- pc: parallel computations, количество запущенных горутин, по умолчанию 10

<!--Запросы-->
## Запросы
### Storage сервер

> /regist_compute 
> POST-запрос, ContentType application/json
> json: {"addr": "*адрес сервера вычислений*"}
> возвращает статус-код
> 
> чтобы сервер вообще мог послать запрос на вычисление, надо зарегестрировать сервер вычислений. но делать это надо не через этот запрос, а через запрос на регистрацию самого вычислительного сервера

> /add_expr
> POST-запрос, ContentType application/json
> json: {"expr": "*выражение*"}
> возвращает id выражения (число)
>
> возвращает id выражения, по запросу /get_result можно получить результат

> /get_result
> GET-запрос
> url-query: ?id=*id выражения*
> возвращает json {"state": "*состояние вычисление*", "result": "*ответ*"}
>
> возвращает состояние вычисления и его результат

> /set_timeout
> POST-запрос, ContentType application/json
> json: {"timeout": {"*символ операции*": *время в миллисекундах*}
> возвращает статус-код
>
> задает время выполнения различных операций. перезаписывает указанные в теле запроса, оставляет без изменений неуказанные. чтобы изменить время ожидания heartbeat'а от сервера вычислений, *символ операции* должен быть "__wait"

> /heart
> GET-запрос
> возвращает статус-код
>
> обновляет время последнего пинга от сервера вычислений, который прислал запрос. если время, которое сервер вычислений не присылал пинг, больше пяти секунд, он считается недоступным.
> если не присылал больше времени, определяемого "__wait" в /set_timeout запросе, сервер вычислений удаляется из списка доступных и его надо заново регистрировать

> /get_compute
> GET-запрос
> возвращает json [{"addr": "*адрес сервера вычислений*", "state": "*состояние*", "last_beat": "*время последнего пинга*"}, ...]
>
> возвращает сервера вычислений и их состояние
