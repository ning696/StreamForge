package com.qidian.userservice.exception;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/**
 * 全局异常处理器。
 * <p>
 * 【@RestControllerAdvice 的作用】
 * 它是一个"横切"的 Bean：所有 Controller 抛出的异常，如果匹配到这里的 {@code @ExceptionHandler}，
 * 就会走对应方法生成响应，而不是让 Spring 用默认的错误页/JSON 返回。
 * 这样业务代码里 Service 层直接抛异常就行，不用每个 Controller 手写 try/catch。
 *
 * <h3>本类处理两类异常</h3>
 * <ol>
 *   <li>{@link BusinessException}：我们自己业务里抛的，带 code/status，按其内容原样返回。</li>
 *   <li>{@link MethodArgumentNotValidException}：Bean Validation 校验失败时 Spring 自动抛出，
 *       统一转成 400 + 校验消息。</li>
 * </ol>
 */
@RestControllerAdvice
public class GlobalExceptionHandler {

    /**
     * 处理业务异常。使用异常里携带的 HttpStatus 和错误码/消息构造响应。
     */
    @ExceptionHandler(BusinessException.class)
    public ResponseEntity<ErrorResponse> handleBusinessException(BusinessException exception) {
        return ResponseEntity
                .status(exception.getStatus())
                .body(new ErrorResponse(exception.getCode(), exception.getMessage()));
    }

    /**
     * 处理请求参数校验失败（@Valid + @NotBlank/@Size 校验不通过）。
     * <p>
     * 取第一个字段错误的 message 作为提示。虽然一次可能有多个字段错，
     * 但对用户来说"分批修正"往往体验更好。如果想返回全部错误，可以改成列表。
     */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ErrorResponse> handleValidation(MethodArgumentNotValidException exception) {
        FieldError fieldError = exception.getBindingResult().getFieldError();
        String message = fieldError == null ? "请求参数错误" : fieldError.getDefaultMessage();
        return ResponseEntity
                .status(HttpStatus.BAD_REQUEST)
                .body(new ErrorResponse("BAD_REQUEST", message));
    }
}
