package com.qidian.userservice.controller;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

/**
 * 健康检查接口。
 * <p>
 * 提供 GET /health，返回 {"status":"UP","service":"user-service"}。
 * 通常给 Kubernetes 的 liveness/readiness probe 或负载均衡器使用，用来判断实例是否还活着。
 *
 * <h3>关键注解</h3>
 * <ul>
 *   <li>{@code @RestController}：等价于 @Controller + @ResponseBody，
 *       每个方法的返回值会被自动序列化成 JSON 写到 HTTP 响应体，而不是像传统 MVC 那样去找 view。</li>
 *   <li>{@code @GetMapping("/health")}：把 GET /health 映射到 health() 方法。</li>
 * </ul>
 */
@RestController
public class HealthController {

    @GetMapping("/health")
    public Map<String, String> health() {
        // Map.of(k1, v1, k2, v2) 是 Java 9+ 提供的"创建不可变 Map"的便捷方法。
        // Spring 的 Jackson 会把它自动序列化成 {"status":"UP","service":"user-service"}。
        return Map.of("status", "UP", "service", "user-service");
    }
}
