package com.qidian.userservice;

import com.qidian.userservice.config.DotEnvLoader;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

import java.nio.file.Path;

/**
 * user-service（用户服务）的启动入口类。
 * <p>
 * 【职责】
 * <ul>
 *   <li>用户注册 / 登录 / 查询这三个 REST 接口</li>
 *   <li>密码用 BCrypt 加密后落库</li>
 *   <li>通过 MyBatis-Plus 操作 MySQL 里的 users 表</li>
 * </ul>
 *
 * <h3>关键注解说明</h3>
 * <ul>
 *   <li>{@code @SpringBootApplication}：Spring Boot 三合一注解，等价于
 *       {@code @Configuration + @EnableAutoConfiguration + @ComponentScan}。
 *       它会自动扫描当前包及其子包下的 Bean（Controller / Service / Component 等）。</li>
 *   <li>{@code @MapperScan("com.qidian.userservice.mapper")}：告诉 MyBatis 去哪个包
 *       扫描 Mapper 接口，自动为它们生成代理实现类。这样我们只写接口就能操作数据库。</li>
 * </ul>
 */
@MapperScan("com.qidian.userservice.mapper")
@SpringBootApplication
public class UserServiceApplication {

    /**
     * JVM 入口。
     * <p>
     * 【顺序很关键】
     * 我们必须先加载 {@code .env}，再调 {@code SpringApplication.run}。
     * 因为 Spring 启动时会读取 application.yml，其中的 {@code ${MYSQL_HOST}} 之类占位符
     * 需要环境变量/系统属性已经就绪。如果先 run 再 load，占位符早就已经解析完了，改也没用。
     *
     * @param args 命令行参数（Spring Boot 会自动把它们绑定到 --key=value 配置里）
     */
    public static void main(String[] args) {
        // 从项目根目录读 .env，把里面的 KEY=VALUE 注册成 Java 的系统属性。
        // 系统属性优先级比 application.yml 里的默认值高，从而实现"本地开发覆盖"。
        DotEnvLoader.load(Path.of(".env"));

        // 启动 Spring Boot。Spring 会：
        //   1. 扫描当前包下所有 @Component/@Service/@Controller 等，实例化并放入容器
        //   2. 起一个内嵌 Tomcat 监听 HTTP 请求
        //   3. 完成数据源、MyBatis 等初始化
        SpringApplication.run(UserServiceApplication.class, args);
    }
}
