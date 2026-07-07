package com.qidian.userservice.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.qidian.userservice.entity.User;
import org.apache.ibatis.annotations.Mapper;

/**
 * 用户表的 MyBatis-Plus Mapper。
 * <p>
 * 【为什么只是一个空接口】
 * MyBatis-Plus 的 {@link BaseMapper} 已经内置了绝大部分单表 CRUD 方法：
 * <ul>
 *   <li>{@code insert(entity)} 插入</li>
 *   <li>{@code selectById(id)} 按主键查</li>
 *   <li>{@code selectOne(wrapper)} 条件查一条</li>
 *   <li>{@code selectCount(wrapper)} 条件计数</li>
 *   <li>{@code updateById / deleteById} 等等</li>
 * </ul>
 * 我们只需要继承它、加上 {@code @Mapper} 注解，Spring/MyBatis 就会自动生成实现类并注入。
 * 复杂查询才需要在这里手写额外方法或者在同名 XML 里写 SQL。
 *
 * <p>{@code @Mapper} 让 MyBatis 识别这是一个 Mapper 接口；配合 {@code @MapperScan}
 * 也可以省略这个注解。此处两个都写了，作用叠加不会出错。
 */
@Mapper
public interface UserMapper extends BaseMapper<User> {
}
