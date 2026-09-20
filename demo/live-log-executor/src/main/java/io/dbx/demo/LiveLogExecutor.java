package io.dbx.demo;

import com.xxl.job.core.context.XxlJobHelper;
import com.xxl.job.core.executor.XxlJobExecutor;
import com.xxl.job.core.handler.IJobHandler;
import java.nio.file.Paths;
import java.time.LocalDateTime;

/** A local ten-minute task that appends one XXL-JOB log line every second. */
public final class LiveLogExecutor {
    private LiveLogExecutor() {}

    public static void main(String[] args) throws Exception {
        final XxlJobExecutor executor = new XxlJobExecutor();
        executor.setAdminAddresses(System.getProperty("demo.admin", "http://127.0.0.1:8080/xxl-job-admin"));
        executor.setAppname(System.getProperty("demo.appname", "dbx-live-log-demo"));
        executor.setPort(Integer.getInteger("demo.port", 9999));
        // The Admin runs in Docker, so its callback address must resolve inside that container.
        executor.setAddress(System.getProperty("demo.address", "http://host.docker.internal:9999/"));
        executor.setAccessToken(System.getProperty("demo.token", ""));
        executor.setLogPath(Paths.get(System.getProperty("user.dir"), "logs").toString());
        executor.setLogRetentionDays(1);
        XxlJobExecutor.registJobHandler("dbxLiveLogDemo", new IJobHandler() {
            @Override public void execute() throws Exception {
                XxlJobHelper.log("DBX live-log demo started: jobId={}, param={}, time={}", XxlJobHelper.getJobId(), XxlJobHelper.getJobParam(), LocalDateTime.now());
                for (int second = 1; second <= 600; second++) {
                    if (Thread.currentThread().isInterrupted()) {
                        XxlJobHelper.log("DBX live-log demo interrupted at second {}", second);
                        return;
                    }
                    XxlJobHelper.log("DBX live-log demo: {}/600 seconds, time={}", second, LocalDateTime.now());
                    Thread.sleep(1000);
                }
                XxlJobHelper.log("DBX live-log demo completed after 10 minutes");
            }
        });
        executor.start();
        Runtime.getRuntime().addShutdownHook(new Thread(executor::destroy));
        System.out.println("DBX live-log executor registered: dbx-live-log-demo / dbxLiveLogDemo");
        Thread.currentThread().join();
    }
}
