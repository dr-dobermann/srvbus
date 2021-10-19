# srvbus

Golang in-memory Service Bus for executing services by demand, and getting the results of their execution.

Provides followed services:

- SrvOutput -- prints message to the console
- SrvPutMessages -- puts messages into the specific queue
- SrvGetMessages -- reads messages from the specific queue

Also consists of an in-memory message server to support SrvPutMessages and SrvGetMessages.

To use message-oriented services MessageServer should be started _manually_ via call _NewMessageServer_.
