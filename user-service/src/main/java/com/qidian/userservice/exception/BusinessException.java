package com.qidian.userservice.exception;

import org.springframework.http.HttpStatus;

/**
 * 业务异常。凡是"业务规则不允许"（账号已存在、用户不存在、参数不合规...）都抛这个。
 * <p>
 * 【为什么继承 RuntimeException】
 * RuntimeException 是"非受检异常"，方法签名不用声明 {@code throws}，
 * Service 层抛出后一路向上传播，最终由 {@link GlobalExceptionHandler} 捕获并转成 HTTP 响应。
 * 如果继承 Exception（受检异常），每个 Controller 方法都要写 throws，非常啰嗦。
 *
 * <h3>三个字段</h3>
 * <ul>
 *   <li>{@code message}（继承自 RuntimeException）：给人看的错误说明。</li>
 *   <li>{@code code}：给程序看的错误码，如 "ACCOUNT_EXISTS"，前端据此做具体逻辑。</li>
 *   <li>{@code status}：期望返回的 HTTP 状态码，如 409 Conflict。</li>
 * </ul>
 */
public class BusinessException extends RuntimeException {

    private final String code;
    private final HttpStatus status;

    /**
     * @param code    业务错误码
     * @param message 给人看的错误消息（会传给父类 RuntimeException）
     * @param status  对应的 HTTP 状态码
     */
    public BusinessException(String code, String message, HttpStatus status) {
        super(message);
        this.code = code;
        this.status = status;
    }

    public String getCode() {
        return code;
    }

    public HttpStatus getStatus() {
        return status;
    }
}
