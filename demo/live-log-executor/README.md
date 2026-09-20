# Local live-log executor

This XXL-JOB 2.3.0 demo registers `dbxLiveLogDemo` with the local Admin and writes one execution log line per second for ten minutes. It is for testing the console's rolling log view.

```powershell
mvn -f demo/live-log-executor/pom.xml package -DskipTests
Set-Location demo/live-log-executor
java -jar target/xxljob-live-log-executor-1.0.0.jar
```

Defaults: Admin `http://127.0.0.1:8080/xxl-job-admin`, AppName `dbx-live-log-demo`, executor port `9999`, advertised address `http://host.docker.internal:9999/`. The advertised address lets the Docker Admin reach the executor running on Windows. The Admin address still uses `127.0.0.1` as requested. Override these with `-Ddemo.admin=...`, `-Ddemo.appname=...`, `-Ddemo.port=...`, `-Ddemo.address=...`, or `-Ddemo.token=...` before `-jar`.

In XXL-JOB Admin, create an executor group with AppName `dbx-live-log-demo` and automatic registration. Create a BEAN job with Handler `dbxLiveLogDemo`, then use **Run once**. The job runs for 600 seconds; its live log is readable while it is running. Stop the executor with Ctrl+C when finished.
