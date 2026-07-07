package com.qidian.userservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * 登录请求体 DTO。参数校验规则和 {@link RegisterRequest} 保持一致，
 * 让前端错填时能得到一致的错误提示。
 * <p>
 * 详见 {@link RegisterRequest} 里对 record 和 Bean Validation 的解释。
 */
public record LoginRequest(
        @NotBlank(message = "账号不能为空")
        @Size(min = 4, max = 32, message = "账号长度必须为4-32")
        String account,

        @NotBlank(message = "密码不能为空")
        @Size(min = 6, max = 64, message = "密码长度必须为6-64")
        String password
) {
}
