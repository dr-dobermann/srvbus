**s2** is a in-memory Service Server implementation for servbus package.

## Introduction

s2 allows to register, run and monitor services. s2 designed to be a part of srvbus package -- a Service Provider in the GoBpm project. s2 could be used separately in case you don't need the whole srvbus functionality.

For logging s2 uses [uber's zap](https://github.com/uber-go/zap) sugared logger. It's only one external dependency of s2.

## ServiceRunner interface

To be executed on ServiceServer any service should implement `ServiceRunner` interface. It consist of only one method `Run`, which gives context and returns error if any. 

If the Service doesn't need to keep state and assumes once-only execution it could be ralized as the function casted to `ServiceFunc`.

    func NewSimpleService(ctx context.Context) error {

        // do something

        return nil
    } 

    id, err := sSrv.AddService("Simple Service", 
        ServiceFunc(NewSimpleService), nil, false)

If it's needed to put parameters into the Service then functor model could be used:

    func NewComplexService(ctx context.Context,
        params ...ServiceParams) (ServiceRunner, error) {
        
        // check parameters validity
        ...

        // create a functor
        complexService := fune(ctx context.Context) error {
            
            // implement service logic here
        }

        // return functor as a ServiceRunner
        return ServiceFunc(complexService), nil
    }

In case the Service should keep state or there is necessity in complex data and logic, the standard model `struct + methods` could be used.

    type newService struct {

        //...

    } 

    func (ns *newService) Run(ctx context.Context) error {

        //

        return nil
    }

    func NewService(ctx context.Context, 
        params ...ServiceParam) (ServiceRunner, error) {

        ns := new(newService)

        // initialize data structure and service

        return ns, nil
    }

Some services could be stopped and resumed. These services should provide stop channel stopCh during their registration over AddService. This channel is used to tell to the Service that it should stop. To stop service the ServiceServer send struct{} into it.

To resume the Service its Run method shoul be called again. This model makes it possible to create streaming services. Services, which don't provide such channel cannot be stopped and resumed.

## Run and Stop the ServiceServer

The ServiceServer is created by the `New` function of s2 package. `New` only demands a non-nil logger pointer. If name and id of the server isn't provided, they will generated.

To start service `Run` method of the ServiceServer should be called.
During the `Run` execution the two goroutines are started:

  1. one processing results of the Services execution
  2. main ServiceServer processing loop

The ServiceSErver doesn't have any stop method. Instead the cancel function of the context could be used.

## Managing Services

### Service registration

Services registered on the ServiceServer by `AddService` method of the ServiceServer. This function creates a new ServiceRecord on the server. The ServiceServer doesn't check if names of Services already exists. Server gives registered service an unique id.

### Service execution

Service could be started right after its registration if in `AddService` parameter `startImmediately` is true.

If the Service is not started after registration it could be started by `ExecService` method of ServiceServer.

### Service stop and resume

As it was mentioned earlier, only services with stopChannel could be stopped and resumed. The ServiceServer sends `struct{}` into the channel to indicate the Service should stop. Service ends it work and return `nil` from its `Run` method.

To resume service the ServiceServer runs it `Run` method. If something should be sent into the Service from outside, it might be done over the different channel or the Service data struct method before calling `ResumeService`.

### Wait for service

The ServiceServer provides method which waits finalization of one or all services running on the Server. If caller provides not nil id (`uuid.Nil`) then function only waits for a single Service. if nil id provided, then method waits for all the running services.

## Service statistics

The ServiceServer has `Stat` method with returns `S2Stat` structure which consists current state of the Server. S2Stat implements `fmt.Stringer` interface so it's easy to get a human-readable representation of the ServiceServer status.
