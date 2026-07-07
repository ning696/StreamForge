package com.qidian.userservice.exception;

/**
 * 错误响应统一结构。
 * <p>
 * 所有出错的 API 响应都会返回这个格式：
 * <pre>{@code
 *   { "code": "ACCOUNT_EXISTS", "message": "账号已存在" }
 * }</pre>
 * 前端根据 {@code code} 做业务分支，用 {@code message} 显示提示。
 * <p>
 * 用 {@code record} 而不是普通类，理由和 DTO 那边一致：字段不变、代码短。
 */
public record ErrorResponse(
        String code,
        String message
) {
}
