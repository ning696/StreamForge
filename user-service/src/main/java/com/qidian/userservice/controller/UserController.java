package com.qidian.userservice.controller;

import com.qidian.userservice.dto.LoginRequest;
import com.qidian.userservice.dto.RegisterRequest;
import com.qidian.userservice.dto.UserResponse;
import com.qidian.userservice.service.UserService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * 用户相关的 REST 接口。
 * <p>
 * 【路由布局】
 * <ul>
 *   <li>{@code POST /api/users/register}：注册新账号</li>
 *   <li>{@code POST /api/users/login}：登录</li>
 *   <li>{@code GET  /api/users/{userId}}：按 ID 查用户</li>
 * </ul>
 *
 * <h3>Controller 的角色</h3>
 * 它是一个"薄壳"：不写业务逻辑，只负责
 * <ol>
 *   <li>把 HTTP 请求参数绑定到 DTO 上</li>
 *   <li>触发 {@code @Valid} 校验</li>
 *   <li>调用 Service 完成业务</li>
 *   <li>把结果原样返回（{@code @RestController} 自动序列化成 JSON）</li>
 * </ol>
 * 异常处理不写在这里——统一由 {@link com.qidian.userservice.exception.GlobalExceptionHandler} 兜底。
 */
@RestController
@RequestMapping("/api/users") // 类级别前缀，方法上的路径拼在它后面
public class UserController {

    private final UserService userService;

    /**
     * 构造函数注入。
     * <p>
     * 【为什么用构造函数而不是 @Autowired 字段注入】
     * <ul>
     *   <li>字段可以声明成 {@code final}，编译器保证不会被重新赋值。</li>
     *   <li>依赖显式列在构造函数上，一眼看清"这个类依赖了什么"。</li>
     *   <li>写单元测试时可以直接 new，不需要 Spring 容器。</li>
     * </ul>
     * 从 Spring 4.3 起，如果类只有一个构造函数，{@code @Autowired} 可以省略。
     */
    public UserController(UserService userService) {
        this.userService = userService;
    }

    /**
     * 注册接口。
     * <p>
     * {@code @Valid}：让 Spring 对 {@code @RequestBody} 里的 {@link RegisterRequest} 执行 Bean Validation。
     *     校验不通过会抛 {@code MethodArgumentNotValidException}，由全局异常处理转成 400。
     * {@code @RequestBody}：把请求体 JSON 反序列化成对象。
     */
    @PostMapping("/register")
    public UserResponse register(@Valid @RequestBody RegisterRequest request) {
        return userService.register(request);
    }

    /**
     * 登录接口。
     */
    @PostMapping("/login")
    public UserResponse login(@Valid @RequestBody LoginRequest request) {
        return userService.login(request);
    }

    /**
     * 按 ID 查用户。
     * <p>
     * {@code @PathVariable}：把 URL 里的 {userId} 占位符绑定到方法参数。
     */
    @GetMapping("/{userId}")
    public UserResponse getUser(@PathVariable Long userId) {
        return userService.getUser(userId);
    }
}
