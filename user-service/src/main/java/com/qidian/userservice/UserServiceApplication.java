package com.qidian.userservice;

import com.qidian.userservice.config.DotEnvLoader;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

import java.nio.file.Path;

@MapperScan("com.qidian.userservice.mapper")
@SpringBootApplication
public class UserServiceApplication {

    public static void main(String[] args) {
        DotEnvLoader.load(Path.of(".env"));
        SpringApplication.run(UserServiceApplication.class, args);
    }

}