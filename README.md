**srvbus** is a serice bus which united of funcitonality of three servers: EventServer, MessageServer and SerivceServer.

# Introduction

Service Bus is the Service Provider for the complex project [gobpm](https://github.com/dr-dobermann/gobpm) -- the BPMN v2. compliant run-time engine on Go.

srvbus provides an event service, e messaging service and an external services invokation service through its EventServer, MessageServer and ServiceServer. All three services could be used separately but their combining could simplify the task solving. For example, instead of checking if a topic is created on the EventServer, the user could subscribe on `NEW_TOPIC_EVT` event to catch a moment of the creation without ineffective topics list pooling.

As it mentioned before, srvbus was written to support the GoBpm project -- BPM-engine for executing BPMN v2.0 comply business processes. Meantime srvbus is a standalone package which could be used in projects demanding light, in-memory and gRPC-enabling services.

There are a lot of effective and robust services with similar or higher functionality, written by other teams, but I need to lower the complexity level on my GoBpm development. 

Now I have simple, functional and robust services with minimal dependencies and minimal footprint. In case I will need more functionality, I could use these service providers as a proxy for more complex, enterprise-ready services.

# Service Bus

Service Bus combines three services, provided by [EventServer](https://github.com/dr-dobermann/srvbus/tree/master/es), [MessageServer](https://github.com/dr-dobermann/srvbus/tree/master/ms) and [ServiceServer](https://github.com/dr-dobermann/srvbus/tree/master/s2).

On ServiceBus creation all three servers creates and ready for running.

## Start and stop the ServiceBus

To run the ServiceBus, `Run` function of the `srvbus` package should be called.

To stop the service bus, cancel function of the context used in `Run` invocation should be used.
There is no another way to stop the service bus and underlaying servers.

## Accesing to Servers

Service Bus does not provide services of underlayed servers by itself. It just give an acces to them throug its methods `GetEventServer`, `GetMessageServer` and `GetServiceServer`. After getting the pointer to a desired server, the user could call the server methods.

## gRPC interface

ServerBus has a gRPC interface handler, which could be accessed from `sbgrpc` package. User could create a handler by package's `New` function and then run it.

The gRPC handler provides three rpc-s:

    rpc GetMessageServer( ServerRequest ) returns ( ServerResponse ) {}

    rpc GetEventServer( ServerRequest ) returns ( ServerResponse ) {}

    rpc GetServiceServer( ServerRequest ) returns ( ServerResponse ) {}

All of them return a gRPC hadler of desired service. 

