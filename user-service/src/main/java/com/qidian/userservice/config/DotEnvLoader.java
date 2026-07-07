package com.qidian.userservice.config;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.function.BiConsumer;
import java.util.function.Function;

/**
 * .env 文件加载工具。
 * <p>
 * 用法：在应用启动时（Spring 还未初始化配置之前）调用 {@link #load(Path)}，
 * 会把 {@code .env} 里的 KEY=VALUE 逐行读出来，注册到 JVM 系统属性里。
 * 之后 Spring 的 application.yml 里 {@code ${MYSQL_HOST}} 之类占位符就能被解析到。
 *
 * <h3>为什么要有这个类</h3>
 * Spring Boot 本身不识别 .env 文件——那是 Node/Python 生态的习惯。
 * 但本地开发时我们不想每次都在 IDEA 里配环境变量，一个和 media-service 一致的 .env
 * 使用体验会舒服很多。所以自己写了这个小加载器。
 *
 * <h3>设计特点</h3>
 * <ul>
 *   <li><b>final 类 + 私有构造函数</b>：工具类的标准写法，防止有人 {@code new DotEnvLoader()}。</li>
 *   <li><b>不覆盖已有值</b>：如果一个 KEY 在系统属性或环境变量里已经存在，就跳过 .env 里的值。
 *       这样线上环境注入的变量永远优先，.env 只是本地兜底。</li>
 *   <li><b>可测试</b>：核心逻辑 {@code load(path, existingValue, setValue)} 把"如何读取已有值""如何写入"
 *       抽成函数参数，测试时可以用 map 替代真的 System.getProperty，从而不污染 JVM 全局状态。</li>
 * </ul>
 */
public final class DotEnvLoader {

    // 私有构造：明确表示"本类不应被实例化，只用静态方法"。
    private DotEnvLoader() {
    }

    /**
     * 生产环境入口：从系统属性/环境变量读已有值，通过 System.setProperty 写入。
     *
     * @param path .env 文件路径（不存在时静默跳过）
     */
    public static void load(Path path) {
        // 【方法引用】DotEnvLoader::existingValue 等价于 key -> existingValue(key)
        load(path, DotEnvLoader::existingValue, (key, value) -> System.setProperty(key, value));
    }

    /**
     * 可测试入口：注入"读取现有值"和"写入"两个函数。
     * <p>
     * 【包私有可见性 (package-private)】没有 public/private/protected 修饰的方法只能在
     * 同一个包内访问。这样对外只暴露简单的 {@link #load(Path)}，测试类放在同包下也能调这个复杂重载。
     *
     * @param path          .env 文件路径
     * @param existingValue 给定 key -> 返回"当前已存在的值"（null 或空表示不存在）
     * @param setValue      写入函数（key, value）
     */
    static void load(Path path, Function<String, String> existingValue, BiConsumer<String, String> setValue) {
        // 文件不存在或不是普通文件（是目录、损坏的符号链接等）就直接返回。
        // 【为什么用 isRegularFile 而不是 exists】"目录"也 exists；我们只对普通文件感兴趣。
        if (!Files.isRegularFile(path)) {
            return;
        }
        // 【try-with-resources】括号里声明的资源在 try 块结束后会自动 close()，
        // 相当于隐式的 finally { reader.close(); }。避免文件句柄泄漏。
        try (BufferedReader reader = Files.newBufferedReader(path)) {
            String line;
            // readLine() 返回 null 表示读到文件末尾。
            while ((line = reader.readLine()) != null) {
                loadLine(line, existingValue, setValue);
            }
        } catch (IOException ex) {
            // IO 异常时抛非受检异常，包装原异常保留堆栈，方便定位问题。
            throw new IllegalStateException("Failed to load .env file: " + path, ex);
        }
    }

    /**
     * 处理单行 .env 内容。
     */
    private static void loadLine(String rawLine, Function<String, String> existingValue, BiConsumer<String, String> setValue) {
        String line = rawLine.trim();
        // 跳过空行和 # 开头的注释行。
        if (line.isEmpty() || line.startsWith("#")) {
            return;
        }
        // 兼容 shell 的 "export KEY=VALUE" 写法：把 export 前缀去掉。
        if (line.startsWith("export ")) {
            line = line.substring("export ".length()).trim();
        }
        // 找第一个 "=" 的位置。用 indexOf 而不是 split("=")，
        // 是为了让 value 里本身包含 "=" 时不被拆坏（比如 base64 编码结尾）。
        int separatorIndex = line.indexOf('=');
        // <=0 意味着：要么没有 '='，要么 '=' 在第一位（key 是空）。两种情况都不合法。
        if (separatorIndex <= 0) {
            return;
        }
        String key = line.substring(0, separatorIndex).trim();
        String value = unquote(line.substring(separatorIndex + 1).trim());
        // key 为空 或 已存在值非空 -> 跳过（不覆盖已有值）。
        if (key.isEmpty() || hasText(existingValue.apply(key))) {
            return;
        }
        setValue.accept(key, value);
    }

    /**
     * 查找 key 现有值：优先系统属性，其次系统环境变量。
     * <p>
     * 【为什么系统属性优先】JVM 启动时可以用 -Dkey=value 显式覆盖，
     * 这种"我明确设了"的意图应该被尊重。
     */
    private static String existingValue(String key) {
        String systemProperty = System.getProperty(key);
        if (hasText(systemProperty)) {
            return systemProperty;
        }
        return System.getenv(key);
    }

    /**
     * 如果值被 " 或 ' 包裹，就去掉两端引号，让 KEY="hello world" 里的空格保留。
     */
    private static String unquote(String value) {
        if (value.length() >= 2) {
            char first = value.charAt(0);
            char last = value.charAt(value.length() - 1);
            if ((first == '"' && last == '"') || (first == '\'' && last == '\'')) {
                return value.substring(1, value.length() - 1);
            }
        }
        return value;
    }

    /**
     * 判断字符串非 null 且去空白后非空。
     * 相当于 Spring 的 StringUtils.hasText，但这里为了减少依赖自己写了一个。
     */
    private static boolean hasText(String value) {
        return value != null && !value.trim().isEmpty();
    }
}
