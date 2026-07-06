package com.qidian.userservice.dto;

public record UserResponse(
        Long userId,
        String account,
        String username
) {
}
