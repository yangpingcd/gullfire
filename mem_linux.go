// +build linux

package main

/*
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>

static long get_mem_available()
{
	FILE* fp = fopen( "/proc/meminfo", "r" );
	if ( fp != NULL )
	{
		size_t bufsize = 1024 * sizeof(char);
		char* buf      = (char*)malloc( bufsize );
		long value     = -1L;
		while ( getline( &buf, &bufsize, fp ) >= 0 )
		{
			//if ( strncmp( buf, "MemTotal", 8 ) != 0 )
			if ( strncmp( buf, "MemFree", 7 ) != 0 )
				continue;
			sscanf( buf, "%*s%ld", &value );
			break;
		}
		fclose( fp );
		free( (void*)buf );
		if ( value != -1L )
			return (size_t)(value * 1024L );
	}
	return 0;
}

static long get_mem_total()
{
	long pages = sysconf(_SC_PHYS_PAGES);
    long page_size = sysconf(_SC_PAGE_SIZE);
    return pages * page_size;
}
*/
/*import "C"


func mem_GetAvailable() uint64 {
	avail := uint64(C.get_mem_available())
	return avail
}

func mem_GetTotal() uint64 {
	total := uint64(C.get_mem_total())
	return total
}
*/

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

//
func mem_GetValue(key string) (value uint64) {
	value = uint64(0)

	file, err := os.OpenFile("/proc/meminfo", 0, 0)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	//re := regexp.MustCompile(`^MemTotal:\s*(\d+)`)
	//re := regexp.MustCompile(`^MemFree:\s*(\d+)`)

	re, err := regexp.Compile(fmt.Sprintf(`^%s:\s*(\d+)`, key))
	if err != nil {
		return
	}

	for scanner.Scan() {
		line := scanner.Text()
		//fmt.Println(line)
		if ss := re.FindStringSubmatch(line); ss != nil {
			//fmt.Println("ss[0]=", ss[0])
			//fmt.Println("ss[1]=", ss[1])
			if tempValue, e := strconv.Atoi(ss[1]); e == nil {
				value = uint64(tempValue)
			}
			break
		}
	}

	return
}

func mem_GetAvailable() uint64 {
	return mem_GetValue("MemFree") * uint64(1024)
}
func mem_GetTotal() uint64 {
	return mem_GetValue("MemTotal") * uint64(1024)
}
