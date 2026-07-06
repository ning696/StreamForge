package com.qidian.userservice.exception;

public record ErrorResponse(
        String code,
        String message
) {
}
