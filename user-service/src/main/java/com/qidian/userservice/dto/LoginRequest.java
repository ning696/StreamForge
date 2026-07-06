package com.qidian.userservice.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record LoginRequest(
        @NotBlank(message = "账号不能为空")
        @Size(min = 4, max = 32, message = "账号长度必须为4-32")
        String account,

        @NotBlank(message = "密码不能为空")
        @Size(min = 6, max = 64, message = "密码长度必须为6-64")
        String password
) {
}
