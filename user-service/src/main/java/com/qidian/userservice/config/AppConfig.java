package com.qidian.userservice.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.web.servlet.config.annotation.CorsRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

/**
 * 应用级别的通用 Bean 配置。
 * <p>
 * 【@Configuration 的意义】
 * Spring 启动时会扫描到这个类，把里面所有 {@code @Bean} 方法的返回值注册进容器。
 * 其他类通过构造函数注入或 {@code @Autowired} 就能拿到这些 Bean。
 *
 * <h3>本类提供的 Bean</h3>
 * <ol>
 *   <li>{@link PasswordEncoder}：密码哈希器，用于 register 时加密、login 时校验。</li>
 *   <li>{@link WebMvcConfigurer}：跨域（CORS）配置，允许前端从别的域名调用我们的 API。</li>
 * </ol>
 */
@Configuration
public class AppConfig {

    /**
     * 密码加密器。使用 BCrypt 算法：
     * <ul>
     *   <li>自带盐（salt）：同一个密码每次加密结果都不同，防彩虹表攻击。</li>
     *   <li>自适应耗时：加密"故意慢"，让暴力破解代价大。</li>
     * </ul>
     * <p>
     * 用法：
     * <pre>{@code
     *   String hash = passwordEncoder.encode("plainPassword");  // 加密
     *   boolean ok = passwordEncoder.matches("plainPassword", hash);  // 校验
     * }</pre>
     * <p>
     * 【为什么定义为接口 PasswordEncoder 而不是具体的 BCryptPasswordEncoder】
     * 依赖抽象，方便未来替换成 Argon2、SCrypt 等其他算法而不用改业务代码。
     */
    @Bean
    public PasswordEncoder passwordEncoder() {
        return new BCryptPasswordEncoder();
    }

    /**
     * CORS 配置。允许 /api/** 下的所有接口被跨域调用。
     * <p>
     * 【为什么需要】前后端分离项目里，前端跑在 :5173 端口，后端跑在 :8080 端口，
     * 浏览器认为它们不同源，默认会拦截请求。加上 CORS 响应头后浏览器就放行了。
     * <p>
     * 【匿名内部类】{@code new WebMvcConfigurer() { ... }} 是在这里"临时实现"一个接口，
     * 因为只用一次不值得单独建一个类文件。
     */
    @Bean
    public WebMvcConfigurer corsConfigurer() {
        return new WebMvcConfigurer() {
            @Override
            public void addCorsMappings(CorsRegistry registry) {
                registry.addMapping("/api/**")                                     // 生效路径
                        .allowedOriginPatterns("*")                                // 允许任意来源
                        .allowedMethods("GET", "POST", "PUT", "DELETE", "OPTIONS") // 允许的 HTTP 方法
                        .allowedHeaders("*");                                      // 允许携带任意请求头
                // 【安全建议】生产环境把 allowedOriginPatterns("*") 换成具体域名，
                // 例如 .allowedOriginPatterns("https://streamforge.example.com")。
            }
        };
    }
}
