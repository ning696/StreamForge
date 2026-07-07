package com.qidian.userservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * 注册请求体 DTO（Data Transfer Object，数据传输对象）。
 * <p>
 * 【为什么用 record】
 * Java 14+ 提供的 {@code record} 关键字自动帮我们生成：
 * <ul>
 *   <li>私有 final 字段（不可变）</li>
 *   <li>全参构造函数</li>
 *   <li>访问器方法（{@code account()}, {@code password()}, {@code username()}）</li>
 *   <li>{@code equals} / {@code hashCode} / {@code toString}</li>
 * </ul>
 * 非常适合"只用来搬数据"的场景，几行代码代替原来 20+ 行的 POJO。
 *
 * <h3>校验注解 (Bean Validation)</h3>
 * Controller 层加了 {@code @Valid} 后，Spring 会在方法调用前自动跑这些校验，
 * 校验失败会抛 {@code MethodArgumentNotValidException}，
 * 被 {@code GlobalExceptionHandler} 捕获后转成 400 响应。
 * 这样业务代码里就不用手写 "if null..." 之类的样板逻辑。
 * <ul>
 *   <li>{@code @NotBlank}：字符串非 null 且去空白后非空</li>
 *   <li>{@code @Size(min, max)}：字符串长度必须在区间内</li>
 * </ul>
 */
public record RegisterRequest(
        @NotBlank(message = "账号不能为空")
        @Size(min = 4, max = 32, message = "账号长度必须为4-32")
        String account,

        @NotBlank(message = "密码不能为空")
        @Size(min = 6, max = 64, message = "密码长度必须为6-64")
        String password,

        @NotBlank(message = "用户名不能为空")
        @Size(min = 1, max = 32, message = "用户名长度必须为1-32")
        String username
) {
}
