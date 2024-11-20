package main

import (
	"fmt"
	"reflect"
	"strings"
)

type User struct {
	Name  string `name:"name"`
	Email string `name:"email"`
}

type Core struct {
	Size int `name:"size"`
}

type Config struct {
	User User `name:"user"`
	Core Core `name:"core"`
}

type MyStruct struct {
	A struct {
		B string `name:"b"`
	} `name:"a"`
}

func parseKeyValue(str string, result any) error {
	parts := strings.Split(str, "=")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format: %s", str)
	}
	keyPath, value := parts[0], parts[1]
	keys := strings.Split(keyPath, ".")

	v := reflect.ValueOf(result)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("result must be a non-nil pointer")
	}
	v = v.Elem()

	for _, key := range keys {
		found := false
		for i := 0; i < v.NumField(); i++ {
			typeField := v.Type().Field(i)
			tag := typeField.Tag.Get("name")
			if tag == key {
				fieldValue := v.Field(i)
				if fieldValue.Kind() == reflect.Struct {
					v = fieldValue // 如果字段是结构体，将它设置为下一次迭代的目标
					found = true
					break
				}
				if fieldValue.CanSet() {
					fieldValue.SetString(value)
					return nil
				}
			}
		}
		if !found {
			return fmt.Errorf("field with name tag '%s' not found", key)
		}
	}

	return nil
}

func main() {
	input := "a.b=value"
	var s MyStruct
	err := parseKeyValue(input, &s)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Result: %+v\n", s) // Output: Result: {A:{B:value}}
}
